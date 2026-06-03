.PHONY: build test lint e2e all qemu-download-kernel qemu-build-initramfs qemu-integration

# k3os-kernel release version used for QEMU integration tests.
# Override via env var: KERNEL_VERSION=v0.112.0 make qemu-integration
KERNEL_VERSION ?= v0.111.0

build:
	CGO_ENABLED=0 go build -o k3os .

test:
	go test -race -covermode=atomic -failfast ./...

lint:
	golangci-lint run ./...

e2e:
	docker build -f e2e/Dockerfile.e2e -t k3os-e2e .
	docker run --rm k3os-e2e

qemu-download-kernel:
	KERNEL_VERSION=$(KERNEL_VERSION) integration/qemu/download-kernel.sh

qemu-build-initramfs: build qemu-download-kernel
	integration/qemu/build-initramfs.sh

qemu-integration: qemu-build-initramfs
	integration/qemu/run-qemu.sh

all: build test lint e2e
