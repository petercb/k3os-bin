//go:build linux

package rc

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClockSyncer records whether SyncRTC was called and returns a
// configurable error.
type mockClockSyncer struct {
	called bool
	err    error
}

func (m *mockClockSyncer) SyncRTC() error {
	m.called = true
	return m.err
}

func TestDoClock_CallsSyncRTC(t *testing.T) {
	orig := clockSyncer
	t.Cleanup(func() { clockSyncer = orig })

	m := &mockClockSyncer{}
	clockSyncer = m

	doClock()

	assert.True(t, m.called, "doClock must call clockSyncer.SyncRTC()")
}

func TestDoClock_ErrorDoesNotPanic(t *testing.T) {
	orig := clockSyncer
	t.Cleanup(func() { clockSyncer = orig })

	m := &mockClockSyncer{err: errors.New("rtc: no device")}
	clockSyncer = m

	// doClock should handle the error gracefully without panicking.
	assert.NotPanics(t, func() { doClock() })
	assert.True(t, m.called, "doClock must call clockSyncer.SyncRTC() even when it errors")
}

func TestDoClock_NoExecCommand(t *testing.T) {
	t.Parallel()

	// Parse rc.go and verify doClock does NOT call exec.Command.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "rc.go", nil, parser.AllErrors)
	require.NoError(t, err, "failed to parse rc.go")

	// Find the doClock function.
	var doClockDecl *ast.FuncDecl
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "doClock" {
			doClockDecl = fn
			break
		}
	}
	require.NotNil(t, doClockDecl, "doClock function must exist in rc.go")

	// Verify it does NOT call exec.Command.
	foundExecCommand := false
	ast.Inspect(doClockDecl.Body, func(n ast.Node) bool {
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
	assert.False(t, foundExecCommand, "doClock must not call exec.Command")
}

func TestDoClock_UsesClockSyncer(t *testing.T) {
	t.Parallel()

	// Parse rc.go and verify doClock references the clockSyncer variable.
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "rc.go", nil, parser.AllErrors)
	require.NoError(t, err, "failed to parse rc.go")

	// Find the doClock function.
	var doClockDecl *ast.FuncDecl
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "doClock" {
			doClockDecl = fn
			break
		}
	}
	require.NotNil(t, doClockDecl, "doClock function must exist in rc.go")

	// Verify it calls clockSyncer.SyncRTC().
	foundSyncRTC := false
	ast.Inspect(doClockDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if ok && ident.Name == "clockSyncer" && sel.Sel.Name == "SyncRTC" {
			foundSyncRTC = true
		}
		return true
	})
	assert.True(t, foundSyncRTC, "doClock must call clockSyncer.SyncRTC()")
}
