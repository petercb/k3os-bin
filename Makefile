.PHONY: build test lint e2e all

build:
	CGO_ENABLED=0 go build -o k3os .

test:
	go test -race -covermode=atomic -failfast ./...

lint:
	golangci-lint run ./...

e2e:
	docker build -f e2e/Dockerfile.e2e -t k3os-e2e .
	docker run --rm --privileged k3os-e2e

all: build test lint e2e
