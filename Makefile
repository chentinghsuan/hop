BINARY_NAME=hop
CMD_DIR=./cmd/hop

.PHONY: all build test install clean help

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -ldflags="-s -w" -o $(BINARY_NAME) $(CMD_DIR)
	@echo "Build successful: ./$(BINARY_NAME)"

test:
	@echo "Running tests..."
	go test -v ./...

INSTALL_DIR ?= $(HOME)/.local/bin

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed successfully to $(INSTALL_DIR)/$(BINARY_NAME)!"

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	find . -name "*:Zone.Identifier" -type f -delete 2>/dev/null || true

help:
	@echo "Available targets:"
	@echo "  make build    - Compile binary to ./$(BINARY_NAME)"
	@echo "  make install  - Install binary to $(INSTALL_DIR)"
	@echo "  make test     - Run all unit tests"
	@echo "  make clean    - Remove built binary and temporary files"
