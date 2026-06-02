package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/petercb/k3os-bin/internal/system"
)

var (
	// SystemConfig is the default system configuration
	SystemConfig = system.RootPath("config.yaml")
	// LocalConfig is the local system configuration
	LocalConfig  = system.LocalPath("config.yaml")
	localConfigs = system.LocalPath("config.d")
	cmdlineFile  = "/proc/cmdline"
)

var readers = []reader{
	readSystemConfig,
	readCmdline,
	readLocalConfig,
	readCloudConfig,
	readUserData,
}

// ToEnv converts a CloudConfig to a list of environment variable strings.
func ToEnv(cfg CloudConfig) ([]string, error) {
	data, err := encodeToMap(&cfg)
	if err != nil {
		return nil, err
	}

	return mapToEnv("", data), nil
}

func encodeToMap(cfg *CloudConfig) (map[string]interface{}, error) {
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func mapToEnv(prefix string, data map[string]interface{}) []string {
	var result []string
	for k, v := range data {
		keyName := strings.ToUpper(prefix + camelToSnake(k))
		if sub, ok := v.(map[string]interface{}); ok {
			subResult := mapToEnv(keyName+"_", sub)
			result = append(result, subResult...)
		} else {
			result = append(result, fmt.Sprintf("%s=%v", keyName, v))
		}
	}
	return result
}

// ReadConfig reads and merges all configuration sources into a CloudConfig.
func ReadConfig() (CloudConfig, error) {
	return readersToObject(append(readers, readLocalConfigs()...)...)
}

func readersToObject(readers ...reader) (CloudConfig, error) {
	result := CloudConfig{
		K3OS: K3OS{
			Install: &Install{},
		},
	}

	data, err := merge(readers...)
	if err != nil {
		return result, err
	}

	return result, decodeToObj(data, &result)
}

type reader func() (map[string]interface{}, error)

func merge(readers ...reader) (map[string]interface{}, error) {
	data := map[string]interface{}{}
	for _, r := range readers {
		newData, err := r()
		if err != nil {
			return nil, err
		}
		if newData == nil {
			continue
		}
		normalizeData(newData)
		var mergeErr error
		data, mergeErr = mergeData(data, newData)
		if mergeErr != nil {
			return nil, mergeErr
		}
	}
	return data, nil
}

func readSystemConfig() (map[string]interface{}, error) {
	return readFile(SystemConfig)
}

func readLocalConfig() (map[string]interface{}, error) {
	return readFile(LocalConfig)
}

func readLocalConfigs() []reader {
	var result []reader

	files, err := os.ReadDir(localConfigs)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return []reader{
			func() (map[string]interface{}, error) {
				return nil, err
			},
		}
	}

	for _, f := range files {
		p := filepath.Join(localConfigs, f.Name())
		result = append(result, func() (map[string]interface{}, error) {
			return readFile(p)
		})
	}

	return result
}

func readFile(path string) (map[string]interface{}, error) {
	f, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	data := map[string]interface{}{}
	if err := yaml.Unmarshal(f, &data); err != nil {
		return nil, err
	}

	return data, nil
}
