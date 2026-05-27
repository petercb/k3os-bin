package ssh

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/petercb/k3os-bin/internal/config"
	"github.com/petercb/k3os-bin/internal/iface"
	"github.com/sirupsen/logrus"
)

const (
	sshDir         = ".ssh"
	authorizedFile = "authorized_keys"
)

// SetAuthorizedKeys writes configured SSH authorized keys for the rancher user.
func SetAuthorizedKeys(cfg *config.CloudConfig, withNet bool, fs iface.FileSystem) error {
	bytes, err := fs.ReadFile("/etc/passwd")
	if err != nil {
		return err
	}
	uid, gid, homeDir, err := findUserHomeDir(bytes, "rancher")
	if err != nil {
		return err
	}
	userSSHDir := path.Join(homeDir, sshDir)
	if _, statErr := fs.Stat(userSSHDir); os.IsNotExist(statErr) {
		if err = fs.MkdirAll(userSSHDir, 0o700); err != nil {
			return err
		}
	} else if statErr != nil {
		return statErr
	}
	if err = fs.Chown(userSSHDir, uid, gid); err != nil {
		return err
	}
	userAuthorizedFile := path.Join(userSSHDir, authorizedFile)
	for _, key := range cfg.SSHAuthorizedKeys {
		if err = authorizeSSHKey(key, userAuthorizedFile, uid, gid, withNet, fs); err != nil {
			logrus.Errorf("failed to authorize SSH key %s: %v", key, err)
		}
	}
	return nil
}

func getKey(key string, withNet bool) (string, error) {
	providers := map[string]string{
		"github": "https://github.com/%s.keys",
		"gitlab": "https://gitlab.com/%s.keys",
		"custom": "%s",
	}

	url, err := url.Parse(key)
	if err != nil || url.Scheme == "" {
		return key, nil
	}

	if !withNet {
		return "", nil
	}

	if providerURL, ok := providers[url.Scheme]; ok {
		key = fmt.Sprintf(providerURL, url.Opaque)
	}

	var resp *http.Response
	for i := 0; i < 10; time.Sleep(time.Second) {
		// network interface(s) can be up before DNS is ready, so let's try up to 10 times
		resp, err = http.Get(key) //nolint:bodyclose
		if err == nil || strings.Contains(err.Error(), "unsupported protocol scheme") {
			break
		}
		i++
	}
	if err != nil {
		return "", err
	}
	if resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if resp.StatusCode/100 > 2 {
		return "", fmt.Errorf("%s %s", resp.Proto, resp.Status)
	}
	bytes, err := io.ReadAll(resp.Body)
	return string(bytes), err
}

func authorizeSSHKey(key, file string, uid, gid int, withNet bool, fs iface.FileSystem) error {
	key, err := getKey(key, withNet)
	if err != nil || key == "" {
		return err
	}

	info, err := fs.Stat(file)
	if os.IsNotExist(err) {
		f, createErr := fs.Create(file)
		if createErr != nil {
			return createErr
		}
		if err = fs.Chmod(file, 0o600); err != nil {
			_ = f.Close()
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		info, err = fs.Stat(file)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	bytes, err := fs.ReadFile(file)
	if err != nil {
		return err
	}
	if !strings.Contains(string(bytes), key) {
		bytes = append(bytes, []byte(key)...)
		bytes = append(bytes, '\n')
	}
	perm := info.Mode().Perm()
	if err = writeFileAtomic(fs, file, bytes, perm); err != nil {
		return err
	}
	return fs.Chown(file, uid, gid)
}

func writeFileAtomic(fs iface.FileSystem, filename string, data []byte, perm os.FileMode) error {
	dir, file := path.Split(filename)
	tempFile, err := fs.CreateTemp(dir, "."+file)
	if err != nil {
		return err
	}
	defer func() {
		_ = fs.Remove(tempFile.Name())
	}()
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := fs.Chmod(tempFile.Name(), perm); err != nil {
		return err
	}
	return fs.Rename(tempFile.Name(), filename)
}

func findUserHomeDir(bytes []byte, username string) (uid, gid int, homeDir string, err error) {
	for _, line := range strings.Split(string(bytes), "\n") {
		if strings.HasPrefix(line, username) {
			split := strings.Split(line, ":")
			if len(split) < 6 {
				break
			}
			uid, err = strconv.Atoi(split[2])
			if err != nil {
				return -1, -1, "", err
			}
			gid, err = strconv.Atoi(split[3])
			if err != nil {
				return -1, -1, "", err
			}
			homeDir = split[5]
		}
	}
	return
}
