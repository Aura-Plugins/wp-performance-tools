APP     = wp-perf
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags "-s -w -X main.version=$(VERSION)"
DIST    = dist

.DEFAULT_GOAL := build

.PHONY: build build-all clean vet test install help

## build: compile for the current OS/arch
build:
	go build $(LDFLAGS) -o $(APP) ./cmd/wp-perf

## build-all: cross-compile for all supported targets
build-all: vet
	@mkdir -p $(DIST)
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(APP)-darwin-arm64      ./cmd/wp-perf
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(APP)-darwin-amd64      ./cmd/wp-perf
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(APP)-linux-amd64       ./cmd/wp-perf
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(APP)-linux-arm64       ./cmd/wp-perf
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(APP)-windows-amd64.exe ./cmd/wp-perf
	@echo "Built $(VERSION) → $(DIST)/"

## install: build and install to /usr/local/bin (override with INSTALL_DIR=...)
install: build
	@INSTALL_DIR=$${INSTALL_DIR:-/usr/local/bin}; \
	mv $(APP) $$INSTALL_DIR/$(APP) && \
	echo "Installed to $$INSTALL_DIR/$(APP)"

## vet: run go vet
vet:
	go vet ./...

## test: run unit tests
test:
	go test ./...

## clean: remove build outputs
clean:
	rm -rf $(DIST) $(APP)

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
