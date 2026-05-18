APP_NAME = mw
CMD_PATH = ./cmd/mw
BIN_DIR = ./bin
INSTALL_DIR = ~/.local/bin

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
	@$(BIN_DIR)/$(APP_NAME) meld

install:
	@echo "📦 Installing to $(INSTALL_DIR)/$(APP_NAME)"
	@mkdir -p $(INSTALL_DIR)
	cp $(BIN_DIR)/$(APP_NAME) $(INSTALL_DIR)/$(APP_NAME)
	@echo "✅ Installed. Run with: $(APP_NAME)"

clean:
	@echo "🧹 Cleaning build output..."
	rm -rf $(BIN_DIR)
