# Replace Network Init with u-root NetInit

## Summary

Replace the `doLoopback()` function in `internal/cli/rc/rc.go` which shells
out to `/sbin/ip` with a single call to u-root's `libinit.NetInit()` which
uses the kernel netlink API directly.

## Rationale

The current implementation runs three `exec.Command` calls to `/sbin/ip`:

1. `ip addr add 127.0.0.1/8 dev lo brd + scope host`
2. `ip route add 127.0.0.0/8 dev lo scope host`
3. `ip link set lo up`

This approach has several downsides:

- Requires `/sbin/ip` to be present in the initramfs
- Shells out to external processes during early boot (slow, fragile)
- Does not use the proper kernel netlink API

On kernel 6.8+, the kernel automatically configures the loopback address
(127.0.0.1/8) and route when the `lo` interface is brought up. Only
bringing the interface up is necessary.

## Approach

Use `github.com/u-root/u-root/pkg/libinit.NetInit()` which:

- Uses `vishvananda/netlink` to bring up the loopback interface via netlink
- Is already maintained as part of the u-root project (already a dependency)
- Follows the same pattern used by other Linux init implementations

The replacement is a single function call with no additional abstraction
needed (KISS principle for early boot code).

## Dependencies

- `github.com/u-root/u-root v0.16.0` (already in go.mod)
- `github.com/vishvananda/netlink` (transitive, pulled in by libinit)

## Testing

Since `libinit.NetInit()` requires root and real network interfaces, we
verify the integration through:

- Build verification (compilation confirms correct API usage)
- Absence of old `/sbin/ip` shell-outs in the code
- Existing integration test coverage at the system level
