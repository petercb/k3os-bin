//go:build linux

package rc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/petercb/k3os-bin/internal/namespace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRcNamespace_NotEmpty(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, rcNamespace)
}

func TestRcNamespace_FirstEntry(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, rcNamespace)
	first := rcNamespace[0]
	assert.Contains(t, first.String(), "proc")
	assert.Contains(t, first.String(), "/proc")
}

func TestRcNamespace_LastEntry(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, rcNamespace)
	last := rcNamespace[len(rcNamespace)-1]
	s := last.String()
	assert.Contains(t, s, "/")
	assert.Contains(t, s, "mount")
}

func TestRcNamespace_ContainsCgroup2Mount(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rcNamespace {
		if m, ok := c.(namespace.Mount); ok {
			if m.FSType == "cgroup2" && m.Target == "/sys/fs/cgroup" {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "rcNamespace should contain a cgroup2 mount at /sys/fs/cgroup")
}

func TestRcNamespace_ContainsDevConsole(t *testing.T) {
	t.Parallel()

	found := false
	for _, c := range rcNamespace {
		if d, ok := c.(namespace.Dev); ok {
			if d.Name == "/dev/console" {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "rcNamespace should contain the /dev/console device entry")
}

func TestDoLoopback_UsesLibinitNetInit(t *testing.T) {
	t.Parallel()

	// Parse rc.go and verify doLoopback calls libinit.NetInit instead of exec.Command
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "rc.go", nil, parser.AllErrors)
	require.NoError(t, err, "failed to parse rc.go")

	// Find the doLoopback function
	var doLoopbackDecl *ast.FuncDecl
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "doLoopback" {
			doLoopbackDecl = fn
			break
		}
	}
	require.NotNil(t, doLoopbackDecl, "doLoopback function must exist in rc.go")

	// Verify it calls libinit.NetInit
	foundNetInit := false
	ast.Inspect(doLoopbackDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if ok && ident.Name == "libinit" && sel.Sel.Name == "NetInit" {
			foundNetInit = true
		}
		return true
	})
	assert.True(t, foundNetInit, "doLoopback must call libinit.NetInit()")

	// Verify it does NOT call exec.Command
	foundExecCommand := false
	ast.Inspect(doLoopbackDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if ok && ident.Name == "exec" && sel.Sel.Name == "Command" {
			foundExecCommand = true
		}
		return true
	})
	assert.False(t, foundExecCommand, "doLoopback must not call exec.Command")
}
