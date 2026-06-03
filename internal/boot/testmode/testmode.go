//go:build linux

// Package testmode provides a post-init state verifier for QEMU integration
// tests. When k3os.test_mode is present on the kernel cmdline, the Verifier
// replaces the final exec /sbin/init to check that bootstrap, mode detection,
// and finalization completed successfully.
package testmode

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

const (
	// ResultStart is the delimiter printed before the JSON results.
	ResultStart = "---TEST_RESULTS_START---"
	// ResultEnd is the delimiter printed after the JSON results.
	ResultEnd = "---TEST_RESULTS_END---"
)

// Check represents a single verification check result.
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// Phase represents a group of related checks.
type Phase struct {
	Name   string  `json:"name"`
	Passed bool    `json:"passed"`
	Checks []Check `json:"checks"`
}

// Results is the top-level test results structure.
type Results struct {
	Passed  bool    `json:"passed"`
	Phases  []Phase `json:"phases"`
	Summary string  `json:"summary"`
}

// Verifier checks post-init system state and outputs structured JSON results.
type Verifier struct {
	StatFunc     func(name string) (os.FileInfo, error)
	ReadFileFunc func(name string) ([]byte, error)
	HostnameFunc func() (string, error)
	RebootFunc   func() error
	Output       io.Writer
}

// Run performs all verification checks, outputs results, and calls RebootFunc.
func (v *Verifier) Run() error {
	out := v.Output
	if out == nil {
		out = os.Stdout
	}

	reboot := v.RebootFunc
	if reboot == nil {
		reboot = func() error {
			return syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
		}
	}

	results := v.verify()

	data, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("testmode: failed to marshal results: %w", err)
	}

	fmt.Fprintln(out, ResultStart)
	fmt.Fprintln(out, string(data))
	fmt.Fprintln(out, ResultEnd)

	return reboot()
}

func (v *Verifier) verify() *Results {
	bootstrapPhase := v.checkBootstrap()
	modePhase := v.checkModeDetection()
	finalizationPhase := v.checkFinalization()

	phases := []Phase{bootstrapPhase, modePhase, finalizationPhase}

	totalChecks := 0
	passedChecks := 0
	allPassed := true
	for i := range phases {
		for _, c := range phases[i].Checks {
			totalChecks++
			if c.Passed {
				passedChecks++
			}
		}
		if !phases[i].Passed {
			allPassed = false
		}
	}

	return &Results{
		Passed:  allPassed,
		Phases:  phases,
		Summary: fmt.Sprintf("%d/%d checks passed", passedChecks, totalChecks),
	}
}

func (v *Verifier) checkBootstrap() Phase {
	checks := []Check{
		v.checkProcMounted(),
		v.checkEtcPopulated(),
	}

	passed := true
	for _, c := range checks {
		if !c.Passed {
			passed = false
			break
		}
	}

	return Phase{
		Name:   "bootstrap",
		Passed: passed,
		Checks: checks,
	}
}

func (v *Verifier) checkModeDetection() Phase {
	checks := []Check{
		v.checkRunK3osExists(),
		v.checkModeFile(),
	}

	passed := true
	for _, c := range checks {
		if !c.Passed {
			passed = false
			break
		}
	}

	return Phase{
		Name:   "mode_detection",
		Passed: passed,
		Checks: checks,
	}
}

func (v *Verifier) checkFinalization() Phase {
	checks := []Check{
		v.checkHostname(),
	}

	passed := true
	for _, c := range checks {
		if !c.Passed {
			passed = false
			break
		}
	}

	return Phase{
		Name:   "finalization",
		Passed: passed,
		Checks: checks,
	}
}

func (v *Verifier) checkProcMounted() Check {
	_, err := v.StatFunc("/proc/self/status")
	if err != nil {
		return Check{
			Name:   "proc_mounted",
			Passed: false,
			Detail: "/proc/self/status does not exist",
		}
	}
	return Check{
		Name:   "proc_mounted",
		Passed: true,
		Detail: "/proc/self/status exists",
	}
}

func (v *Verifier) checkEtcPopulated() Check {
	data, err := v.ReadFileFunc("/etc/passwd")
	if err != nil {
		return Check{
			Name:   "etc_populated",
			Passed: false,
			Detail: fmt.Sprintf("failed to read /etc/passwd: %v", err),
		}
	}
	if !strings.Contains(string(data), "rancher") {
		return Check{
			Name:   "etc_populated",
			Passed: false,
			Detail: "/etc/passwd does not contain rancher user",
		}
	}
	return Check{
		Name:   "etc_populated",
		Passed: true,
		Detail: "/etc/passwd contains rancher",
	}
}

func (v *Verifier) checkRunK3osExists() Check {
	_, err := v.StatFunc("/run/k3os")
	if err != nil {
		return Check{
			Name:   "run_k3os_exists",
			Passed: false,
			Detail: "/run/k3os directory does not exist",
		}
	}
	return Check{
		Name:   "run_k3os_exists",
		Passed: true,
		Detail: "/run/k3os directory exists",
	}
}

func (v *Verifier) checkModeFile() Check {
	data, err := v.ReadFileFunc("/run/k3os/mode")
	if err != nil {
		return Check{
			Name:   "mode_file",
			Passed: false,
			Detail: fmt.Sprintf("failed to read /run/k3os/mode: %v", err),
		}
	}

	mode := strings.TrimSpace(string(data))
	validModes := map[string]bool{
		"local": true,
		"live":  true,
		"disk":  true,
	}
	if !validModes[mode] {
		return Check{
			Name:   "mode_file",
			Passed: false,
			Detail: fmt.Sprintf("/run/k3os/mode contains invalid mode: %q", mode),
		}
	}
	return Check{
		Name:   "mode_file",
		Passed: true,
		Detail: fmt.Sprintf("/run/k3os/mode contains valid mode: %q", mode),
	}
}

func (v *Verifier) checkHostname() Check {
	hostname, err := v.HostnameFunc()
	if err != nil {
		return Check{
			Name:   "hostname_set",
			Passed: false,
			Detail: fmt.Sprintf("failed to get hostname: %v", err),
		}
	}
	if hostname == "" {
		return Check{
			Name:   "hostname_set",
			Passed: false,
			Detail: "hostname is empty",
		}
	}
	return Check{
		Name:   "hostname_set",
		Passed: true,
		Detail: fmt.Sprintf("hostname is %q", hostname),
	}
}
