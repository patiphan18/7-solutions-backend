.PHONY: run build test vet fmt check lint lint-proto gen-proto
.DEFAULT_GOAL := run

BAZEL_FLAGS = $(if $(BAZEL_CONFIG),--config=$(BAZEL_CONFIG),)
GOLANGCI_LINT ?= golangci-lint

lint:
	$(GOLANGCI_LINT) run

lint-proto:
	bazel test //proto/... $(BAZEL_FLAGS)

gen-proto:
	bash scripts/gen-proto.sh $(BAZEL_FLAGS)

run:
	go run ./cmd

build:
	go build -o .cache/user-management-api ./cmd

test:
	go test ./... -cover

vet:
	go vet ./...

fmt:
	$(GOLANGCI_LINT) fmt
	$(GOLANGCI_LINT) run --fix

check: test vet lint
	@unformatted=$$(gofmt -l cmd internal pkg scripts) && test -z "$$unformatted"
