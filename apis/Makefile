CRD_OUTPUT_DIR ?= ../platform/crds

.PHONY: generate
generate:
	controller-gen object:headerFile="" paths="./..."

.PHONY: manifests
manifests:
	controller-gen crd paths="./policy.stackrox.io/..." output:crd:dir=$(CRD_OUTPUT_DIR)

.PHONY: test
test:
	go test ./...

.PHONY: all
all: generate manifests test
