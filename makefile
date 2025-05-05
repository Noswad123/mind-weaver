APP_NAME = mw
CMD_PATH = ./cmd/incantation
BIN_DIR = ./bin
INSTALL_DIR = ~/.dotfiles/bin
LOOM_PATH = scripts/loom/main.py

.PHONY: all build watch clean install visualize

all: build install

build:
	@echo "🔨 Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) $(CMD_PATH)
	@echo "✅ Built at $(BIN_DIR)/$(APP_NAME)"

watch:
	@$(BIN_DIR)/$(APP_NAME) --banish --gaze

format:
	@$(BIN_DIR)/$(APP_NAME) --engrave

install:
	@echo "📦 Installing to $(INSTALL_DIR)/$(APP_NAME)"
	@mkdir -p $(INSTALL_DIR)
	cp $(BIN_DIR)/$(APP_NAME) $(INSTALL_DIR)/$(APP_NAME)
	@echo "✅ Installed. Run with: $(APP_NAME)"

venv:
	@py -m venv .venv
	@.venv/bin/pip install -r scripts/loom/requirements.txt
	@echo "✅ Virtual environment initialized. Run with: source .venv/bin/activate"

visualize:
	@echo "🌐 Launching note graph visualization..."
	py $(LOOM_PATH)

clean:
	@echo "🧹 Cleaning build output..."
	rm -rf $(BIN_DIR)
