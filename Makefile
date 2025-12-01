SHELL := /bin/bash
BIN_DIR := bin
BINARY := $(BIN_DIR)/dock-it
CLEAN_TARGETS := $(BIN_DIR) dock-it dist

.PHONY: build test clean

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) ./cmd/dock-it

test:
	go test ./...

clean:
	rm -rf $(CLEAN_TARGETS)
	go clean
