package enterchroot

import (
	"fmt"
	"os"

	"github.com/petercb/k3os-bin/internal/mount"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func mountProc() error {
	if ok, err := mount.Mounted("/proc"); ok && err == nil {
		return nil
	}
	logrus.Debug("mkdir /proc")
	if err := os.MkdirAll("/proc", 0755); err != nil {
		return err
	}
	logrus.Debug("mount /proc")
	return mount.ForceMount("proc", "/proc", "proc", "")
}

func mountDev() error {
	if files, err := os.ReadDir("/dev"); err == nil && len(files) > 2 {
		return nil
	}
	logrus.Debug("mkdir /dev")
	if err := os.MkdirAll("/dev", 0755); err != nil {
		return err
	}
	logrus.Debug("mounting /dev")
	return mount.ForceMount("none", "/dev", "devtmpfs", "")
}

func mknod(path string, mode uint32, major, minor int) error {
	if fi, err := os.Stat(path); err == nil {
		logrus.Debugf("mknod: %s exists [%s,%d]", path, fi.Name(), fi.Mode())
		return nil
	}

	dev := int((major << 8) | (minor & 0xff) | ((minor & 0xfff00) << 12))
	logrus.Debugf("mknod %s", path)
	return unix.Mknod(path, mode, dev)
}

func ensureloop() error {
	// CONFIG_BLK_DEV_LOOP should be set to 'y' in the kernel
	if err := mountProc(); err != nil {
		return errors.Wrapf(err, "failed to mount proc")
	}
	if err := mountDev(); err != nil {
		return errors.Wrapf(err, "failed to mount dev")
	}

	if err := mknod("/dev/loop-control", 0700|unix.S_IFCHR, 10, 237); err != nil {
		return err
	}
	for i := 0; i < 7; i++ {
		if err := mknod(fmt.Sprintf("/dev/loop%d", i), 0700|unix.S_IFBLK, 7, i); err != nil {
			return err
		}
	}

	return nil
}
