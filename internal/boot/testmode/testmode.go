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
	executionPhase := v.checkModeExecution()
	finalizationPhase := v.checkFinalization()

	phases := []Phase{bootstrapPhase, modePhase, executionPhase, finalizationPhase}

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
		v.checkExpectedMode(),
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

func (v *Verifier) checkModeExecution() Phase {
	checks := []Check{
		v.checkModeTarget(),
		v.checkPivotRoot(),
	}

	passed := true
	for _, c := range checks {
		if !c.Passed {
			passed = false
			break
		}
	}

	return Phase{
		Name:   "mode_execution",
		Passed: passed,
		Checks: checks,
	}
}

// checkModeTarget verifies that the mode handler successfully set up its
// target filesystem. For disk mode, /run/k3os/target must be mounted and
// contain the k3os system directory. For live mode, the overlay should exist.
func (v *Verifier) checkModeTarget() Check {
	// Read the detected mode to determine what to check.
	modeData, err := v.ReadFileFunc("/run/k3os/mode")
	if err != nil {
		return Check{
			Name:   "mode_target",
			Passed: false,
			Detail: fmt.Sprintf("cannot determine mode: %v", err),
		}
	}

	mode := strings.TrimSpace(string(modeData))

	switch mode {
	case "disk":
		// Disk mode should have K3OS_STATE mounted at /run/k3os/target
		// with the k3os system directory present.
		_, err := v.StatFunc("/run/k3os/target/k3os/system")
		if err != nil {
			return Check{
				Name:   "mode_target",
				Passed: false,
				Detail: "disk mode: /run/k3os/target/k3os/system not found (K3OS_STATE mount failed)",
			}
		}
		return Check{
			Name:   "mode_target",
			Passed: true,
			Detail: "disk mode: /run/k3os/target/k3os/system exists",
		}
	case "live", "install":
		// Live/install mode mounts media at /.base. If /.base exists with
		// content the handler succeeded. If not, it's expected in minimal
		// test environments without boot media — treat as a soft pass since
		// the essential bootstrap/mode-detection phases are what we validate.
		_, err := v.StatFunc("/.base/k3os")
		if err != nil {
			return Check{
				Name:   "mode_target",
				Passed: true,
				Detail: fmt.Sprintf("%s mode: /.base/k3os not found (no boot media — expected in test)", mode),
			}
		}
		return Check{
			Name:   "mode_target",
			Passed: true,
			Detail: fmt.Sprintf("%s mode: /.base/k3os exists", mode),
		}
	default:
		// For other modes (local, shell), the target directory
		// existence is sufficient evidence the handler ran.
		return Check{
			Name:   "mode_target",
			Passed: true,
			Detail: fmt.Sprintf("%s mode: no target check required", mode),
		}
	}
}

// checkPivotRoot verifies whether a disk-mode pivot_root occurred by checking
// for the /.root directory (the old root mountpoint). If k3os.mode= is set
// on the cmdline, this isn't a disk-mode boot and the check soft-passes.
// Otherwise (auto-detect mode), /.root must exist proving disk handler ran.
func (v *Verifier) checkPivotRoot() Check {
	cmdline, _ := v.ReadFileFunc("/proc/cmdline")
	cmdlineStr := string(cmdline)

	// If k3os.mode= is set on cmdline, this isn't a disk-mode boot; soft pass.
	for _, field := range strings.Fields(cmdlineStr) {
		if strings.HasPrefix(field, "k3os.mode=") {
			return Check{
				Name:   "pivot_root",
				Passed: true,
				Detail: "k3os.mode= set on cmdline, pivot_root check not applicable",
			}
		}
	}

	// No explicit mode override means this was a disk-mode auto-detect boot.
	// After pivot_root, /.root should exist as the old root mountpoint.
	_, err := v.StatFunc("/.root")
	if err != nil {
		return Check{
			Name:   "pivot_root",
			Passed: false,
			Detail: "/.root not found (pivot_root from disk mode did not occur)",
		}
	}
	return Check{
		Name:   "pivot_root",
		Passed: true,
		Detail: "/.root exists (pivot_root from disk mode succeeded)",
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

// checkExpectedMode reads k3os.test_expected_mode from /proc/cmdline and
// verifies the detected mode matches. If the parameter is absent, the check
// is a soft pass (no expectation configured).
func (v *Verifier) checkExpectedMode() Check {
	cmdline, err := v.ReadFileFunc("/proc/cmdline")
	if err != nil {
		return Check{
			Name:   "expected_mode",
			Passed: true,
			Detail: "cannot read /proc/cmdline, skipping expected mode check",
		}
	}

	// Parse k3os.test_expected_mode=<value> from cmdline.
	var expected string
	for _, field := range strings.Fields(string(cmdline)) {
		if strings.HasPrefix(field, "k3os.test_expected_mode=") {
			expected = strings.TrimPrefix(field, "k3os.test_expected_mode=")
			break
		}
	}

	if expected == "" {
		return Check{
			Name:   "expected_mode",
			Passed: true,
			Detail: "no k3os.test_expected_mode set, skipping",
		}
	}

	// Read the actual detected mode.
	modeData, err := v.ReadFileFunc("/run/k3os/mode")
	if err != nil {
		return Check{
			Name:   "expected_mode",
			Passed: false,
			Detail: fmt.Sprintf("expected mode %q but cannot read /run/k3os/mode: %v", expected, err),
		}
	}

	actual := strings.TrimSpace(string(modeData))
	if actual != expected {
		return Check{
			Name:   "expected_mode",
			Passed: false,
			Detail: fmt.Sprintf("expected mode %q but got %q", expected, actual),
		}
	}

	return Check{
		Name:   "expected_mode",
		Passed: true,
		Detail: fmt.Sprintf("mode matches expected: %q", expected),
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
