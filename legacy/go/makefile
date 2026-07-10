APP_NAME = mw
CMD_PATH = ./cmd/mw
BIN_DIR = ./bin
PREFIX ?= $(HOME)/.local
INSTALL_DIR ?= $(PREFIX)/bin

.PHONY: all build test smoke format install clean

all: build install

build:
	@echo "🔨 Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) $(CMD_PATH)
	@echo "✅ Built at $(BIN_DIR)/$(APP_NAME)"

test:
	go test ./...

format:
	gofmt -w cmd internal

smoke:
	scripts/smoke-fresh-install.sh

install:
	@echo "📦 Installing to $(INSTALL_DIR)/$(APP_NAME)"
	@mkdir -p $(INSTALL_DIR)
	cp $(BIN_DIR)/$(APP_NAME) $(INSTALL_DIR)/$(APP_NAME)
	@echo "✅ Installed. Run with: $(APP_NAME)"

clean:
	@echo "🧹 Cleaning build output..."
	rm -rf $(BIN_DIR)
