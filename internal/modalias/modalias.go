package modalias

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/moby/pkg/parsers/kernel"
	"github.com/ryanuber/go-glob"
	"pault.ag/go/modprobe"
)

type ModuleAliases struct {
	aliases map[string]string
}

func getAliasesFile() (string, error) {
	kernelVersion, err := kernel.GetKernelVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get kernel version: %w", err)
	}
	kernelDir := fmt.Sprintf("/lib/modules/%s", kernelVersion)
	if _, err := os.Stat(kernelDir); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s not found", kernelDir)
	}
	return filepath.Join(kernelDir, "modules.alias"), nil
}

func Init() (ModuleAliases, error) {
	filename, err := getAliasesFile()
	if err != nil {
		return ModuleAliases{}, err
	}

	lookupTable := make(map[string]string)

	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		// aliases file doesn't exist, so return empty lookupTable
		return ModuleAliases{aliases: lookupTable}, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return ModuleAliases{}, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip lines that don't start with "alias"
		if !strings.HasPrefix(line, "alias ") {
			continue
		}

		// Split the line into fields
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// The alias pattern is the second field, and the module name is the
		// third field
		aliasPattern := fields[1]
		moduleName := fields[2]

		lookupTable[aliasPattern] = moduleName
	}

	if err := scanner.Err(); err != nil {
		return ModuleAliases{}, fmt.Errorf("error reading file: %w", err)
	}

	return ModuleAliases{aliases: lookupTable}, nil
}

func (mod ModuleAliases) Lookup(name string) string {
	for pattern, v := range mod.aliases {
		if glob.Glob(pattern, name) {
			return v
		}
	}
	return name
}

func (mod ModuleAliases) Load(alias string) error {
	return modprobe.Load(mod.Lookup(alias), "")
}
