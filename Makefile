GO ?= go
BIN_DIR ?= bin
BINARY ?= mysqloperatorcalculator
GOLANGCI_LINT_VERSION ?= v2.12.2

.PHONY: all
all: fmt vet lint test build

.PHONY: build
build:
	$(GO) build -o $(BIN_DIR)/$(BINARY) ./src

.PHONY: test
test:
	$(GO) test -race -coverprofile=coverage.out ./...

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: lint
lint:
	golangci-lint run

.PHONY: lint-install
lint-install:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) coverage.out