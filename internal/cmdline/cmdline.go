// Package cmdline wraps u-root pkg/cmdline behind the iface.CmdlineParser
// interface for dependency injection and testability.
package cmdline

import (
	"strings"
	"unicode"

	"github.com/petercb/k3os-bin/internal/iface"
	uroot "github.com/u-root/u-root/pkg/cmdline"
)

// parser wraps a u-root CmdLine struct, implementing iface.CmdlineParser.
type parser struct {
	cl *uroot.CmdLine
}

// New returns a CmdlineParser backed by /proc/cmdline (via u-root).
func New() iface.CmdlineParser {
	return &parser{cl: uroot.NewCmdLine()}
}

// NewFromString constructs a CmdlineParser from an arbitrary raw kernel
// command-line string. This is useful for testing without /proc/cmdline.
func NewFromString(raw string) iface.CmdlineParser {
	return &parser{cl: &uroot.CmdLine{
		Raw:   raw,
		AsMap: parseToMap(raw),
	}}
}

// Flag returns the value of a kernel cmdline flag and whether it was set.
func (p *parser) Flag(name string) (string, bool) {
	return p.cl.Flag(name)
}

// Contains reports whether the named flag is present on the kernel cmdline.
func (p *parser) Contains(name string) bool {
	return p.cl.ContainsFlag(name)
}

// Consoles returns all console= TTY device names from the raw cmdline.
func (p *parser) Consoles() []string {
	return p.cl.Consoles()
}

// Raw returns the full unparsed kernel command-line string.
func (p *parser) Raw() string {
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
