package config

import (
	"fmt"
	"io"

	"github.com/ghodss/yaml"
	"github.com/rancher/mapper/convert"
)

// PrintInstall marshals the install portion of a CloudConfig to YAML bytes.
func PrintInstall(cfg CloudConfig) ([]byte, error) {
	data, err := convert.EncodeToMap(cfg.K3OS.Install)
	if err != nil {
		return nil, err
	}

	toYAMLKeys(data)
	return yaml.Marshal(data)
}

// Write serializes a CloudConfig to YAML and writes it to the given writer.
func Write(cfg CloudConfig, writer io.Writer) error {
	bytes, err := ToBytes(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal [%s]: %w", string(bytes), err)
	}
	_, err = writer.Write(bytes)
	return err
}

// ToBytes serializes a CloudConfig to YAML bytes, excluding install settings.
func ToBytes(cfg CloudConfig) ([]byte, error) {
	cfg.K3OS.Install = nil
	data, err := convert.EncodeToMap(cfg)
	if err != nil {
		return nil, err
	}

	toYAMLKeys(data)
	return yaml.Marshal(data)
}

func toYAMLKeys(data map[string]interface{}) {
	for k, v := range data {
		if sub, ok := v.(map[string]interface{}); ok {
			toYAMLKeys(sub)
		}
		newK := convert.ToYAMLKey(k)
		if newK != k {
			delete(data, k)
			data[newK] = v
		}
	}
}
