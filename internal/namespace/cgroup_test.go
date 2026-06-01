//go:build linux

package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCgroupMounts_String(t *testing.T) {
	t.Parallel()

	c := CgroupMounts{}
	assert.Equal(t, "cgroup-mounts{/sys/fs/cgroup/*}", c.String())
}
