package config

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// PrintInstall marshals the install portion of a CloudConfig to YAML bytes.
func PrintInstall(cfg CloudConfig) ([]byte, error) {
	if cfg.K3OS.Install == nil {
		return nil, nil
	}
	return yaml.Marshal(cfg.K3OS.Install)
}

// Write serializes a CloudConfig to YAML and writes it to the given writer.
func Write(cfg CloudConfig, writer io.Writer) error {
	bytes, err := ToBytes(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	_, err = writer.Write(bytes)
	return err
}

// ToBytes serializes a CloudConfig to YAML bytes, excluding install settings.
func ToBytes(cfg CloudConfig) ([]byte, error) {
	cfg.K3OS.Install = nil
	return yaml.Marshal(cfg)
}
