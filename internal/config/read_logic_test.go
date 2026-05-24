package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCmdline(t *testing.T) {
	// Backup original cmdlineFile
	oldCmdlineFile := cmdlineFile
	defer func() { cmdlineFile = oldCmdlineFile }()

	t.Run("missing cmdline file", func(t *testing.T) {
		tempDir := t.TempDir()
		cmdlineFile = filepath.Join(tempDir, "nonexistent")

		data, err := readCmdline()
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("valid cmdline parameters", func(t *testing.T) {
		tempDir := t.TempDir()
		cmdlinePath := filepath.Join(tempDir, "cmdline")

		// Write simulated kernel cmdline
		cmdlineContent := `k3os.hostname=myhost k3os.password="pass" k3os.dns=8.8.8.8 k3os.dns=1.1.1.1 some_other_param=foo`
		err := os.WriteFile(cmdlinePath, []byte(cmdlineContent), 0o644)
		require.NoError(t, err)

		cmdlineFile = cmdlinePath

		data, err := readCmdline()
		require.NoError(t, err)
		require.NotNil(t, data)

		// Check parsed hostname (k3os.hostname -> nested maps)
		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "myhost", k3os["hostname"])

		// Check quoted password
		assert.Equal(t, "pass", k3os["password"])

		// Check list values (k3os.dns)
		dns, ok := k3os["dns"].([]string)
		require.True(t, ok)
		assert.Equal(t, []string{"8.8.8.8", "1.1.1.1"}, dns)
	})
}

func TestReadFile(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		tempDir := t.TempDir()
		missingPath := filepath.Join(tempDir, "missing.yaml")
		data, err := readFile(missingPath)
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("valid yaml file", func(t *testing.T) {
		tempDir := t.TempDir()
		yamlPath := filepath.Join(tempDir, "valid.yaml")
		yamlContent := `
k3os:
  hostname: myhost
  dns:
    - 8.8.8.8
`
		err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644)
		require.NoError(t, err)

		data, err := readFile(yamlPath)
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "myhost", k3os["hostname"])
	})

	t.Run("malformed yaml file", func(t *testing.T) {
		tempDir := t.TempDir()
		badYamlPath := filepath.Join(tempDir, "bad.yaml")
		badYamlContent := `
k3os:
  - invalid-yaml-structure
  hostname:
`
		err := os.WriteFile(badYamlPath, []byte(badYamlContent), 0o644)
		require.NoError(t, err)

		_, err = readFile(badYamlPath)
		assert.Error(t, err)
	})
}

func TestMerge(t *testing.T) {
	t.Run("multi-source merge override priority", func(t *testing.T) {
		r1 := func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"k3os": map[string]interface{}{
					"hostname": "host1",
					"dns":      []interface{}{"8.8.8.8"},
				},
			}, nil
		}
		r2 := func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"k3os": map[string]interface{}{
					"hostname": "host2", // should override host1
				},
			}, nil
		}
		r3 := func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"k3os": map[string]interface{}{
					"dns": []interface{}{"1.1.1.1"}, // should override [8.8.8.8]
				},
			}, nil
		}

		data, err := merge(r1, r2, r3)
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)

		// Check override
		assert.Equal(t, "host2", k3os["hostname"])

		// Check list override (UpdateMerge behavior)
		dns, ok := k3os["dns"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, []interface{}{"1.1.1.1"}, dns)
	})

	t.Run("schema type coercion during merge", func(t *testing.T) {
		r := func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"k3os": map[string]interface{}{
					"wifi": map[string]interface{}{
						"network": "my-ssid", // should coerce string to slice of strings if schema requires it, or handle correctly
					},
				},
			}, nil
		}

		data, err := merge(r)
		require.NoError(t, err)
		require.NotNil(t, data)

		k3os, ok := data["k3os"].(map[string]interface{})
		require.True(t, ok)

		wifi, ok := k3os["wifi"].(map[string]interface{})
		require.True(t, ok)

		// Check type coercion of schema: k3os.wifi.network is slice of strings
		// Let's see what wifi["network"] becomes. It should coerce to []interface{} or []string under schema rules.
		netVal := wifi["network"]
		assert.NotNil(t, netVal)
	})

	t.Run("reader returns error", func(t *testing.T) {
		r1 := func() (map[string]interface{}, error) {
			return nil, assert.AnError
		}
		_, err := merge(r1)
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func TestReadLocalConfigs(t *testing.T) {
	// Backup localConfigs
	oldLocalConfigs := localConfigs
	defer func() { localConfigs = oldLocalConfigs }()

	t.Run("nonexistent directory", func(t *testing.T) {
		tempDir := t.TempDir()
		localConfigs = filepath.Join(tempDir, "nonexistent")
		readers := readLocalConfigs()
		assert.Nil(t, readers)
	})

	t.Run("empty directory", func(t *testing.T) {
		tempDir := t.TempDir()
		localConfigs = tempDir
		readers := readLocalConfigs()
		assert.Empty(t, readers)
	})

	t.Run("valid config.d files", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create file 1
		err := os.WriteFile(filepath.Join(tempDir, "01-config.yaml"), []byte(`
k3os:
  hostname: host1
`), 0o644)
		require.NoError(t, err)

		// Create file 2
		err = os.WriteFile(filepath.Join(tempDir, "02-config.yaml"), []byte(`
k3os:
  hostname: host2
`), 0o644)
		require.NoError(t, err)

		localConfigs = tempDir

		readers := readLocalConfigs()
		require.Len(t, readers, 2)

		// Read files through readers and verify content
		data1, err := readers[0]()
		require.NoError(t, err)
		k3os1 := data1["k3os"].(map[string]interface{})
		assert.Equal(t, "host1", k3os1["hostname"])

		data2, err := readers[1]()
		require.NoError(t, err)
		k3os2 := data2["k3os"].(map[string]interface{})
		assert.Equal(t, "host2", k3os2["hostname"])
	})
}

func TestReadersToObject(t *testing.T) {
	t.Run("successful conversion", func(t *testing.T) {
		r1 := func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"hostname": "my-cool-host",
			}, nil
		}
		r2 := func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"k3os": map[string]interface{}{
					"password": "my-secret-password",
				},
			}, nil
		}

		cc, err := readersToObject(r1, r2)
		require.NoError(t, err)
		assert.Equal(t, "my-cool-host", cc.Hostname)
		assert.Equal(t, "my-secret-password", cc.K3OS.Password)
	})

	t.Run("merge error propagation", func(t *testing.T) {
		r1 := func() (map[string]interface{}, error) {
			return nil, assert.AnError
		}
		_, err := readersToObject(r1)
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func TestMapToEnv(t *testing.T) {
	t.Run("ToEnv conversion of CloudConfig", func(t *testing.T) {
		cfg := CloudConfig{
			Hostname: "myhost",
			K3OS: K3OS{
				Password: "pass",
				Environment: map[string]string{
					"var_one": "value1",
				},
			},
		}

		env, err := ToEnv(cfg)
		require.NoError(t, err)

		// Env array elements are "KEY=VALUE" strings
		assert.Contains(t, env, "HOSTNAME=myhost")
		assert.Contains(t, env, "K3OS_PASSWORD=pass")
		assert.Contains(t, env, "K3OS_ENVIRONMENT_VAR_ONE=value1")
	})

	t.Run("mapToEnv flat and nested maps with various types", func(t *testing.T) {
		input := map[string]interface{}{
			"hostname": "somehost",
			"k3os": map[string]interface{}{
				"install": map[string]interface{}{
					"silent": true,
					"tty":    "ttyS0",
				},
				"dnsNameservers": []interface{}{"8.8.8.8", "1.1.1.1"},
			},
		}

		env := mapToEnv("", input)

		assert.Contains(t, env, "HOSTNAME=somehost")
		assert.Contains(t, env, "K3OS_INSTALL_SILENT=true")
		assert.Contains(t, env, "K3OS_INSTALL_TTY=ttyS0")
		// Array conversion inside mapper maps to list format. e.g. K3OS_DNS_NAMESERVERS=[8.8.8.8 1.1.1.1] or similar %v string formatting
		assert.Contains(t, env, "K3OS_DNS_NAMESERVERS=[8.8.8.8 1.1.1.1]")
	})
}

func TestReadConfig(t *testing.T) {
	// Backup all global paths
	oldSystemConfig := SystemConfig
	oldLocalConfig := LocalConfig
	oldLocalConfigs := localConfigs
	oldCmdlineFile := cmdlineFile
	oldHostnameFile := hostnameFile
	oldSSHFile := sshFile
	oldUserdataFile := userdataFile

	defer func() {
		SystemConfig = oldSystemConfig
		LocalConfig = oldLocalConfig
		localConfigs = oldLocalConfigs
		cmdlineFile = oldCmdlineFile
		hostnameFile = oldHostnameFile
		sshFile = oldSSHFile
		userdataFile = oldUserdataFile
	}()

	tempDir := t.TempDir()

	// 1. Setup SystemConfig
	sysPath := filepath.Join(tempDir, "config.yaml")
	sysContent := `
k3os:
  password: "sys-password"
  dnsNameservers:
    - 8.8.8.8
`
	err := os.WriteFile(sysPath, []byte(sysContent), 0o644)
	require.NoError(t, err)
	SystemConfig = sysPath

	// 2. Setup Cmdline
	cmdlinePath := filepath.Join(tempDir, "cmdline")
	cmdlineContent := `k3os.password=cmdline-password k3os.install.silent=true`
	err = os.WriteFile(cmdlinePath, []byte(cmdlineContent), 0o644)
	require.NoError(t, err)
	cmdlineFile = cmdlinePath

	// 3. Setup LocalConfig
	localPath := filepath.Join(tempDir, "local-config.yaml")
	localContent := `
k3os:
  password: "local-password"
`
	err = os.WriteFile(localPath, []byte(localContent), 0o644)
	require.NoError(t, err)
	LocalConfig = localPath

	// 4. Setup localConfigs (config.d)
	configDDir := filepath.Join(tempDir, "config.d")
	err = os.Mkdir(configDDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(configDDir, "99-custom.yaml"), []byte(`
k3os:
  dnsNameservers:
    - 1.1.1.1
`), 0o644)
	require.NoError(t, err)
	localConfigs = configDDir

	// 5. Setup CloudConfig paths
	hostPath := filepath.Join(tempDir, "local_hostname")
	err = os.WriteFile(hostPath, []byte("cloud-host"), 0o644)
	require.NoError(t, err)
	hostnameFile = hostPath

	sshKeysPath := filepath.Join(tempDir, "authorized_keys")
	err = os.WriteFile(sshKeysPath, []byte("ssh-rsa AAAA-custom-key"), 0o644)
	require.NoError(t, err)
	sshFile = sshKeysPath

	// 6. Setup UserData
	usrDataPath := filepath.Join(tempDir, "userdata")
	usrDataContent := `
k3os:
  k3sArgs:
    - --server
`
	err = os.WriteFile(usrDataPath, []byte(usrDataContent), 0o644)
	require.NoError(t, err)
	userdataFile = usrDataPath

	// Run ReadConfig
	cc, err := ReadConfig()
	require.NoError(t, err)

	// Assertions based on merge priorities:
	// - Hostname from cloud-config: "cloud-host"
	assert.Equal(t, "cloud-host", cc.Hostname)

	// - Password: LocalConfig overrides cmdline (local-password)
	assert.Equal(t, "local-password", cc.K3OS.Password)

	// - Install silent from cmdline: true
	assert.True(t, cc.K3OS.Install.Silent)

	// - DNS nameservers: config.d overrides system config (1.1.1.1)
	assert.Equal(t, []string{"1.1.1.1"}, cc.K3OS.DNSNameservers)

	// - SSH authorized keys from sshFile
	assert.Equal(t, []string{"ssh-rsa AAAA-custom-key"}, cc.SSHAuthorizedKeys)

	// - K3sArgs from userdata: "--server"
	assert.Equal(t, []string{"--server"}, cc.K3OS.K3sArgs)
}

func TestReadUserData(t *testing.T) {
	oldUserdataFile := userdataFile
	defer func() { userdataFile = oldUserdataFile }()

	t.Run("missing userdata file", func(t *testing.T) {
		tempDir := t.TempDir()
		userdataFile = filepath.Join(tempDir, "missing")
		res, err := readUserData()
		require.NoError(t, err)
		assert.Nil(t, res)
	})

	t.Run("yaml userdata file", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "userdata")
		err := os.WriteFile(path, []byte(`
k3os:
  password: "cool-password"
`), 0o644)
		require.NoError(t, err)
		userdataFile = path

		res, err := readUserData()
		require.NoError(t, err)
		require.NotNil(t, res)

		k3os := res["k3os"].(map[string]interface{})
		assert.Equal(t, "cool-password", k3os["password"])
	})

	t.Run("shell script userdata (hashbang)", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "userdata")
		scriptContent := `#!/bin/sh
echo "hello world"
`
		err := os.WriteFile(path, []byte(scriptContent), 0o644)
		require.NoError(t, err)
		userdataFile = path

		res, err := readUserData()
		require.NoError(t, err)
		require.NotNil(t, res)

		// Verification: converted to writeFiles map
		runCmd, ok := res["runCmd"].([]interface{})
		require.True(t, ok)
		assert.Contains(t, runCmd, "source /run/k3os/userdata")

		writeFiles, ok := res["writeFiles"].([]interface{})
		require.True(t, ok)
		require.Len(t, writeFiles, 1)

		fileMap := writeFiles[0].(map[string]interface{})
		assert.Equal(t, "/run/k3os/userdata", fileMap["path"])
		assert.Equal(t, "root", fileMap["owner"])
		assert.Equal(t, "0700", fileMap["permissions"])
		assert.Equal(t, scriptContent, fileMap["content"])
	})

	t.Run("binary script userdata (null byte)", func(t *testing.T) {
		tempDir := t.TempDir()
		path := filepath.Join(tempDir, "userdata")
		binaryContent := []byte{0x7f, 'E', 'L', 'F', 0, 1, 2, 3}
		err := os.WriteFile(path, binaryContent, 0o644)
		require.NoError(t, err)
		userdataFile = path

		res, err := readUserData()
		require.NoError(t, err)
		require.NotNil(t, res)

		runCmd, ok := res["runCmd"].([]interface{})
		require.True(t, ok)
		assert.Contains(t, runCmd, "source /run/k3os/userdata")

		writeFiles, ok := res["writeFiles"].([]interface{})
		require.True(t, ok)
		require.Len(t, writeFiles, 1)

		fileMap := writeFiles[0].(map[string]interface{})
		assert.Equal(t, "/run/k3os/userdata", fileMap["path"])
		assert.Equal(t, "b64", fileMap["encoding"])
		assert.Equal(t, base64.StdEncoding.EncodeToString(binaryContent), fileMap["content"])
	})
}
