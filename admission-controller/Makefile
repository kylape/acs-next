.PHONY: build image push clean test test-unit test-integration

REGISTRY ?= registry.dev-registry.svc:5000
IMAGE_NAME ?= acs-next/admission-controller
TAG ?= dev

build:
	go build -o bin/admission-controller ./cmd/admission-controller

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

mod-tidy:
	go mod tidy
