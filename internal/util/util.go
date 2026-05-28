package util

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
)

// WriteFileAtomic writes data to a file atomically using a temporary file and rename.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir, file := path.Split(filename)
	tempFile, err := os.CreateTemp(dir, "."+file)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tempFile.Name()) }()
	if _, err := tempFile.Write(data); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempFile.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tempFile.Name(), filename)
}

// HTTPDownloadToFile downloads a URL and writes the response body to a file atomically.
func HTTPDownloadToFile(url, dest string) error {
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	return WriteFileAtomic(dest, body, 0o644)
}

// HTTPLoadBytes fetches a URL and returns the response body as bytes.
func HTTPLoadBytes(url string) ([]byte, error) {
	var resp *http.Response
	resp, err := http.Get(url)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("non-200 http response: %d", resp.StatusCode)
		}

		var bytes []byte
		bytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return bytes, nil
	}

	return nil, err
}

// ExistsAndExecutable returns true if the path exists and has executable permissions.
func ExistsAndExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	mode := info.Mode().Perm()
	return mode&os.ModePerm != 0
}

// RunScript executes a script file, detecting whether it has a shebang or should use /bin/sh.
func RunScript(path string, arg ...string) error {
	if !ExistsAndExecutable(path) {
		return nil
	}

	script, err := os.Open(path)
	if err != nil {
		return err
	}

	magic := make([]byte, 2)
	if _, err = script.Read(magic); err != nil {
		return err
	}

	cmd := exec.Command("/bin/sh", path)
	if string(magic) == "#!" {
		cmd = exec.Command(path, arg...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// EnsureDirectoryExists creates the directory if it does not exist, or returns an error if the path is not a directory.
func EnsureDirectoryExists(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
	} else {
		err = os.MkdirAll(dir, 0o755)
		if err != nil {
			return err
		}
	}
	return nil
}
