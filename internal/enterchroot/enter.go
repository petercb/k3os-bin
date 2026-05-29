//go:build linux
// +build linux

package enterchroot

import (
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/moby/sys/reexec"
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/petercb/k3os-bin/internal/loopdev"
	"github.com/petercb/k3os-bin/internal/mount"
	"golang.org/x/sys/unix"
)

const (
	magic = "_SQMAGIC_"
)

var (
	symlinks = []string{"lib", "bin", "sbin"}
	// DebugCmdline is the kernel cmdline parameter that enables debug logging.
	DebugCmdline        = ""
	procFilesystemsPath = "/proc/filesystems"
	// loopAttacher is the default LoopAttacher implementation; override in tests.
	loopAttacher iface.LoopAttacher = loopdev.NewAttacher()
)

// Enter the k3OS root
func Enter() {
	if os.Getenv("ENTER_DEBUG") == "true" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
	}

	setResourceLimit(unix.RLIMIT_NOFILE, 1048576, 1048576)
	setResourceLimit(unix.RLIMIT_NPROC, unix.RLIM_INFINITY, unix.RLIM_INFINITY)
	// Increase mlocks to avoid Go crashes
	// https://github.com/golang/go/wiki/LinuxKernelSignalVectorBug
	setResourceLimit(unix.RLIMIT_MEMLOCK, 67108864, 67108864)

	slog.Debug("running bootstrap")
	err := run(os.Getenv("ENTER_DATA"))
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
}

func setResourceLimit(resource int, current, maximum uint64) {
	lim := unix.Rlimit{Cur: current, Max: maximum}
	err := unix.Setrlimit(resource, &lim)
	if err != nil {
		log.Printf("Failed to set rlimit %x: %v", resource, err)
	}
}

func isDebug() bool {
	if os.Getenv("ENTER_DEBUG") == "true" {
		return true
	}

	if DebugCmdline == "" {
		return false
	}

	bytes, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		// ignore error
		return false
	}
	for _, word := range strings.Fields(string(bytes)) {
		if word == DebugCmdline {
			return true
		}
	}

	return false
}

// Mount sets up the k3OS root filesystem and executes the enter-root process.
func Mount(dataDir string, args []string, stdout, stderr io.Writer) error {
	if err := ensureloop(); err != nil {
		return err
	}

	if isDebug() {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
		_ = os.Setenv("ENTER_DEBUG", "true")
	}

	root, offset, err := findRoot()
	if err != nil {
		return err
	}

	_ = os.Setenv("ENTER_DATA", dataDir)
	_ = os.Setenv("ENTER_ROOT", root)

	slog.Debug("using data and root", "data", dataDir, "root", root)

	stat, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("failed to find %s: %w", root, err)
	}

	if !stat.IsDir() {
		slog.Debug("attaching file", "root", root, "offset", offset)
		dev, err := loopAttacher.Attach(root, offset, true)
		if err != nil {
			return fmt.Errorf("creating loopback device: %w", err)
		}
		defer func() {
			if dderr := dev.Detach(); dderr != nil {
				slog.Error("failed detaching file", "root", root, "offset", offset, "error", dderr)
			}
		}()
		_ = os.Setenv("ENTER_DEVICE", dev.Path())

		go func() {
			// Assume that after 3 seconds loop back device has been mounted
			time.Sleep(3 * time.Second)
			if err := dev.SetAutoclear(); err != nil {
				slog.Error("failed to set autoclear", "device", dev.Path(), "error", err)
			}
		}()
	}

	slog.Debug("running enter-root", "args", os.Args[1:])
	if os.Getpid() == 1 {
		if err := syscall.Exec(os.Args[0], append([]string{"enter-root"}, args[1:]...), os.Environ()); err != nil {
			return fmt.Errorf("failed to exec enter-root: %w", err)
		}
	}
	cmd := &exec.Cmd{
		Path: os.Args[0],
		Args: append([]string{"enter-root"}, args[1:]...),
		SysProcAttr: &syscall.SysProcAttr{
			Cloneflags:   syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
			Unshareflags: syscall.CLONE_NEWNS,
			Pdeathsig:    syscall.SIGKILL,
		},
		Stdout: stdout,
		Stdin:  os.Stdin,
		Stderr: stderr,
		Env:    os.Environ(),
	}
	return cmd.Run()
}

func findRoot() (string, uint64, error) {
	root := os.Getenv("ENTER_ROOT")
	if root != "" {
		return root, 0, nil
	}

	for _, suffix := range []string{".root", ".squashfs"} {
		test := os.Args[0] + suffix
		if _, err := os.Stat(test); err == nil {
			return test, 0, nil
		}
	}

	return inFile()
}

func inFile() (string, uint64, error) {
	f, err := os.Open(reexec.Self())
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 8192)
	test := []byte(strings.ToLower(magic))
	testLength := len(test)
	offset := uint64(0)
	found := 0

	for {
		n, err := f.Read(buf)
		if err == io.EOF && n == 0 {
			break
		} else if err != nil {
			return "", 0, err
		}

		for _, b := range buf[:n] {
			if b == test[found] {
				found++
				if found == testLength {
					return reexec.Self(), offset + 1, nil
				}
			} else {
				found = 0
			}
			offset++
		}
	}

	return "", 0, fmt.Errorf("failed to find image in file %s", os.Args[0])
}

func run(data string) error {
	// TODO: replace github.com/moby/pkg/mountinfo
	mounted, err := mount.Mounted(data)
	if err != nil {
		return fmt.Errorf("checking %s mounted: %w", data, err)
	}

	if !mounted {
		if err = os.MkdirAll(data, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", data, err)
		}
		if err = mount.Mount(data, data, "none", "rbind"); err != nil {
			return fmt.Errorf("remounting data %s: %w", data, err)
		}
	}

	root := os.Getenv("ENTER_ROOT")
	device := os.Getenv("ENTER_DEVICE")

	slog.Debug("using root", "root", root, "device", device)

	usr := filepath.Join(data, "usr")
	dotRoot := filepath.Join(data, ".base")

	for _, d := range []string{usr, dotRoot} {
		if err = os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to make dir %s: %w", data, err)
		}
	}

	if device == "" {
		slog.Debug("bind mounting", "src", root, "dst", usr)
		if mountErr := mount.Mount(root, usr, "none", "bind"); mountErr != nil {
			return fmt.Errorf("failed to bind mount: %w", mountErr)
		}
	} else {
		slog.Debug("mounting squashfs", "device", device, "dst", usr)
		squashErr := checkSquashfs()
		err = mount.Mount(device, usr, "squashfs", "ro")
		if err != nil {
			err = fmt.Errorf("mounting squashfs: %w", err)
			if squashErr != nil {
				err = fmt.Errorf("%s: %w", squashErr.Error(), err)
			}
			return err
		}
	}

	if err := os.Chdir(data); err != nil {
		return err
	}

	for _, p := range symlinks {
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			if err := os.Symlink(filepath.Join("usr", p), p); err != nil {
				return fmt.Errorf("failed to symlink %s: %w", p, err)
			}
		}
	}

	slog.Debug("pivoting to . .base")
	if err := syscall.PivotRoot(".", ".base"); err != nil {
		return fmt.Errorf("pivot_root failed: %w", err)
	}

	if err := mount.ForceMount("", ".", "none", "rprivate"); err != nil {
		return fmt.Errorf("making . private %s: %w", data, err)
	}

	if err := syscall.Chroot("/"); err != nil {
		return err
	}

	if err := os.Chdir("/"); err != nil {
		return err
	}

	if _, err := os.Stat("/usr/init"); err != nil {
		return fmt.Errorf("failed to find /usr/init: %w", err)
	}

	_ = os.Unsetenv("ENTER_ROOT")
	_ = os.Unsetenv("ENTER_DATA")
	_ = os.Unsetenv("ENTER_DEVICE")
	return syscall.Exec("/usr/init", os.Args, os.Environ())
}

func checkSquashfs() error {
	if !inProcFS() {
		return errors.New("this kernel does not support squashfs, please enable. " +
			"On Fedora you may need to run \"dnf install kernel-modules-$(uname -r)\"")
	}

	return nil
}

func inProcFS() bool {
	bytes, err := os.ReadFile(procFilesystemsPath)
	if err != nil {
		slog.Error("failed to read filesystem list", "path", procFilesystemsPath, "error", err)
		return false
	}
	return strings.Contains(string(bytes), "squashfs")
}
