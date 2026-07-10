
APP_NAME = mw
BIN_DIR = ./bin
INSTALL_DIR = ~/.local/bin

.PHONY: build check fmt install legacy-go-build

build:
	@echo "🔨 Building Rust $(APP_NAME)..."
	cargo build --workspace

check:
	cargo check --workspace

fmt:
	cargo fmt --all

install: build
	@echo "📦 Installing to $(INSTALL_DIR)/$(APP_NAME)"
	@mkdir -p $(INSTALL_DIR)
	cp target/debug/$(APP_NAME) $(INSTALL_DIR)/$(APP_NAME)
	@echo "✅ Installed. Run with: $(APP_NAME)"

legacy-go-build:
	$(MAKE) -C legacy/go build
