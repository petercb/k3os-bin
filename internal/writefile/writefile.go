// Package writefile handles writing configuration files to disk.
package writefile

import (
	"fmt"
	"log/slog"
	"path"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/petercb/k3os-bin/internal/util"
)

// WriteFiles writes all configured write_files entries.
func WriteFiles(cfg *config.CloudConfig, fs iface.FileSystem, cmd iface.CommandRunner) {
	for i, f := range cfg.WriteFiles {
		c, err := util.DecodeContent(f.Content, f.Encoding)
		if err != nil {
			slog.Error("failed to decode content from write_files item", "index", i, "error", err)
			continue
		}
		f.Content = string(c)
		f.Encoding = ""
		p, err := WriteFile(&f, "/", fs, cmd)
		if err != nil {
			slog.Error("failed to write file", "path", p, "error", err)
			continue
		}
		slog.Info("wrote file to filesystem", "path", p)
	}
}

// WriteFile writes one decoded cloud-config file entry below root.
func WriteFile(f *config.File, root string, fs iface.FileSystem, cmd iface.CommandRunner) (string, error) {
	if f.Encoding != "" {
		return "", fmt.Errorf("unable to write file with encoding %s", f.Encoding)
	}
	p := path.Join(root, f.Path)
	d := path.Dir(p)
	slog.Info("writing file", "dir", d)
	if err := ensureDirectoryExists(fs, d); err != nil {
		return "", err
	}
	perm, err := f.Permissions()
	if err != nil {
		return "", err
	}
	var tmp iface.File
	// create a temporary file in the same directory to ensure it's on the same filesystem
	if tmp, err = fs.CreateTemp(d, "wfs-temp"); err != nil {
		return "", err
	}
	if _, err := tmp.Write([]byte(f.Content)); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	// ensure the permissions are as requested (since WriteFile can be affected by sticky bit)
	if err := fs.Chmod(tmp.Name(), perm); err != nil {
		return "", err
	}
	if f.Owner != "" {
		// we shell out since we don't have a way to look up unix groups natively
		if err := cmd.Run("chown", f.Owner, tmp.Name()); err != nil {
			return "", err
		}
	}
	if err := fs.Rename(tmp.Name(), p); err != nil {
		return "", err
	}
	return p, nil
}

func ensureDirectoryExists(fs iface.FileSystem, dir string) error {
	info, err := fs.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
	} else {
		err = fs.MkdirAll(dir, 0o755)
		if err != nil {
			return err
		}
	}
	return nil
}
