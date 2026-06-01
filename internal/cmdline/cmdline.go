// Package cmdline wraps u-root pkg/cmdline behind the iface.CmdlineParser
// interface for dependency injection and testability.
package cmdline

import (
	"strings"
	"unicode"

	"github.com/petercb/k3os-bin/internal/iface"
	uroot "github.com/u-root/u-root/pkg/cmdline"
)

// procParser reads /proc/cmdline fresh on every method call.
// This is necessary because the parser may be constructed before /proc is
// mounted (e.g., at package init time or in early boot), and we must always
// get the current state of /proc/cmdline when it is finally available.
type procParser struct{}

// New returns a CmdlineParser that reads /proc/cmdline on every method call.
// This ensures correct behavior even when the parser is constructed before
// /proc is mounted.
func New() iface.CmdlineParser {
	return &procParser{}
}

// Flag returns the value of a kernel cmdline flag and whether it was set.
func (p *procParser) Flag(name string) (string, bool) {
	return uroot.NewCmdLine().Flag(name)
}

// Contains reports whether the named flag is present on the kernel cmdline.
func (p *procParser) Contains(name string) bool {
	return uroot.NewCmdLine().ContainsFlag(name)
}

// Consoles returns all console= TTY device names from the raw cmdline.
func (p *procParser) Consoles() []string {
	return uroot.NewCmdLine().Consoles()
}

// Raw returns the full unparsed kernel command-line string.
func (p *procParser) Raw() string {
	return uroot.NewCmdLine().Raw
}

// staticParser uses a pre-built CmdLine, suitable for NewFromString where
// the content is fixed and does not depend on /proc.
type staticParser struct {
	cl *uroot.CmdLine
}

// NewFromString constructs a CmdlineParser from an arbitrary raw kernel
// command-line string. This is useful for testing without /proc/cmdline.
func NewFromString(raw string) iface.CmdlineParser {
	return &staticParser{
		cl: &uroot.CmdLine{
			Raw:   raw,
			AsMap: parseToMap(raw),
		},
	}
}

// Flag returns the value of a kernel cmdline flag and whether it was set.
func (p *staticParser) Flag(name string) (string, bool) {
	return p.cl.Flag(name)
}

// Contains reports whether the named flag is present on the kernel cmdline.
func (p *staticParser) Contains(name string) bool {
	return p.cl.ContainsFlag(name)
}

// Consoles returns all console= TTY device names from the raw cmdline.
func (p *staticParser) Consoles() []string {
	return p.cl.Consoles()
}

// Raw returns the full unparsed kernel command-line string.
func (p *staticParser) Raw() string {
	return p.cl.Raw
}

// parseToMap replicates u-root's unexported parseToMap logic: splits on
// whitespace (respecting quoted fields), then on the first '=' per token.
func parseToMap(raw string) map[string]string {
	m := make(map[string]string)
	lastQuote := rune(0)
	for _, flag := range strings.FieldsFunc(raw, func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)
		}
	}) {
		if len(flag) == 0 {
			continue
		}
		var key, value string
		if idx := strings.Index(flag, "="); idx == -1 {
			key = flag
			value = "1"
		} else {
			key = flag[:idx]
			value = flag[idx+1:]
		}
		canonicalKey := strings.ReplaceAll(key, "-", "_")
		m[canonicalKey] = value
		m[key] = value
	}
	return m
}
