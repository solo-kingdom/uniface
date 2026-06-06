.PHONY: build up down tidy

BIN_DIR := bin
CMDS := lab-kv lab-config lab-lb lab-queue lab-dag lab-ui
COMPOSE_PROFILES ?= all

build:
	@mkdir -p $(BIN_DIR)
	@for cmd in $(CMDS); do \
		echo ">>> building $$cmd"; \
		go build -o $(BIN_DIR)/$$cmd ./cmd/$$cmd; \
	done

tidy:
	go mod tidy

up: build
	docker compose --profile $(COMPOSE_PROFILES) up -d
	@mkdir -p $(BIN_DIR)/pids
	@LAB_CONFIG=configs/default.yaml nohup ./$(BIN_DIR)/lab-kv serve > $(BIN_DIR)/lab-kv.log 2>&1 & echo $$! > $(BIN_DIR)/pids/lab-kv.pid
	@LAB_CONFIG=configs/default.yaml nohup ./$(BIN_DIR)/lab-config serve > $(BIN_DIR)/lab-config.log 2>&1 & echo $$! > $(BIN_DIR)/pids/lab-config.pid
	@LAB_CONFIG=configs/default.yaml nohup ./$(BIN_DIR)/lab-lb serve > $(BIN_DIR)/lab-lb.log 2>&1 & echo $$! > $(BIN_DIR)/pids/lab-lb.pid
	@LAB_CONFIG=configs/default.yaml nohup ./$(BIN_DIR)/lab-queue serve > $(BIN_DIR)/lab-queue.log 2>&1 & echo $$! > $(BIN_DIR)/pids/lab-queue.pid
	@LAB_CONFIG=configs/default.yaml nohup ./$(BIN_DIR)/lab-dag serve > $(BIN_DIR)/lab-dag.log 2>&1 & echo $$! > $(BIN_DIR)/pids/lab-dag.pid
	@LAB_CONFIG=configs/default.yaml nohup ./$(BIN_DIR)/lab-ui > $(BIN_DIR)/lab-ui.log 2>&1 & echo $$! > $(BIN_DIR)/pids/lab-ui.pid
	@echo "Dashboard: http://localhost:3000"

down:
	@for cmd in lab-kv lab-config lab-lb lab-queue lab-dag lab-ui; do \
		if [ -f $(BIN_DIR)/pids/$$cmd.pid ]; then \
			kill $$(cat $(BIN_DIR)/pids/$$cmd.pid) 2>/dev/null || true; \
			rm -f $(BIN_DIR)/pids/$$cmd.pid; \
		fi; \
	done
	docker compose down
