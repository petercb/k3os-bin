# Antigravity IDE Usage

These instructions are specific to sandboxed development using Antigravity IDE and override instructions elsewhere concerning how to build, test, and run commands.

A Devcontainer is used for this development (see .devcontainer).

## Container Commands

Local Docker socket permissions are restricted. Execute all commands inside the target container via SSH instead of the devcontainer CLI:

```sh
ssh -o StrictHostKeyChecking=no root@<container-name-or-ip> "<command>"
```
