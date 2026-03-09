.PHONY: build image push clean test test-unit test-integration

REGISTRY ?= registry.dev-registry.svc:5000
IMAGE_NAME ?= acs-next/broker
TAG ?= dev

build:
	go build -o bin/broker ./cmd/broker

image:
	podman build -t $(REGISTRY)/$(IMAGE_NAME):$(TAG) .

push: image
	podman push $(REGISTRY)/$(IMAGE_NAME):$(TAG)

clean:
	rm -rf bin/

test: test-unit test-integration

test-unit:
	go test -v -race ./internal/...

test-integration:
	go test -v -race ./test/integration/...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

mod-tidy:
	go mod tidy
