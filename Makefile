.PHONY: build test lint e2e all qemu-download-kernel qemu-build-initramfs qemu-integration

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
	integration/qemu/download-kernel.sh

qemu-build-initramfs: build qemu-download-kernel
	integration/qemu/build-initramfs.sh

qemu-integration: qemu-build-initramfs
	integration/qemu/run-qemu.sh

all: build test lint e2e
