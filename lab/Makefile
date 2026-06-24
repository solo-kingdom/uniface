BIN_DIR := bin
COMPOSE_PROFILES ?= all

comma := ,
empty :=
space := $(empty) $(empty)

# 域注册表：module -> binary, compose profiles（空表示无外部中间件）
MODULES := kv config lb queue dag ui

MODULE_kv_BIN := lab-kv
MODULE_kv_PROFILES := kv
MODULE_kv_PORT := 8081

MODULE_config_BIN := lab-config
MODULE_config_PROFILES := config
MODULE_config_PORT := 8082

MODULE_lb_BIN := lab-lb
MODULE_lb_PROFILES :=
MODULE_lb_PORT := 8083

MODULE_queue_BIN := lab-queue
MODULE_queue_PROFILES := queue
MODULE_queue_PORT := 8084

MODULE_dag_BIN := lab-dag
MODULE_dag_PROFILES :=
MODULE_dag_PORT := 8085

MODULE_ui_BIN := lab-ui
MODULE_ui_PROFILES :=
MODULE_ui_PORT := 3000

# LAB_MODULES: all | 逗号/空格分隔的域列表
LAB_MODULES ?= all

ifeq ($(LAB_MODULES),all)
SELECTED_MODULES := $(MODULES)
else
SELECTED_MODULES := $(strip $(subst $(comma),$(space),$(LAB_MODULES)))
endif

ifeq ($(SELECTED_MODULES),$(MODULES))
UP_PROFILES := all
else
UP_PROFILES := $(sort $(foreach m,$(SELECTED_MODULES),$(MODULE_$(m)_PROFILES)))
endif

.PHONY: build up down tidy \
	$(foreach m,$(MODULES),build-$(m) up-$(m) down-$(m))

define build-module-cmd
echo ">>> building $(MODULE_$(1)_BIN)"; \
go build -o $(BIN_DIR)/$(MODULE_$(1)_BIN) ./cmd/$(MODULE_$(1)_BIN);
endef

define start-module-cmd
echo ">>> starting $(MODULE_$(1)_BIN) (log: $(BIN_DIR)/$(MODULE_$(1)_BIN).log)"; \
mkdir -p $(BIN_DIR)/pids; \
if [ "$(1)" = "ui" ]; then \
	LAB_CONFIG=configs/default.yaml nohup ./$(BIN_DIR)/$(MODULE_$(1)_BIN) > $(BIN_DIR)/$(MODULE_$(1)_BIN).log 2>&1 & echo $$$$! > $(BIN_DIR)/pids/$(MODULE_$(1)_BIN).pid; \
else \
	LAB_CONFIG=configs/default.yaml nohup ./$(BIN_DIR)/$(MODULE_$(1)_BIN) serve > $(BIN_DIR)/$(MODULE_$(1)_BIN).log 2>&1 & echo $$$$! > $(BIN_DIR)/pids/$(MODULE_$(1)_BIN).pid; \
fi;
endef

define echo-module-url-cmd
if [ "$(1)" = "ui" ]; then \
	echo "Dashboard: http://localhost:$(MODULE_$(1)_PORT)"; \
else \
	echo "API: http://localhost:$(MODULE_$(1)_PORT)/api/status"; \
fi;
endef

define down-module-cmd
if [ -f $(BIN_DIR)/pids/$(MODULE_$(1)_BIN).pid ]; then \
	kill `cat $(BIN_DIR)/pids/$(MODULE_$(1)_BIN).pid` 2>/dev/null || true; \
	rm -f $(BIN_DIR)/pids/$(MODULE_$(1)_BIN).pid; \
fi;
endef

define module-targets
.PHONY: build-$(1) up-$(1) down-$(1)
build-$(1):
	@mkdir -p $(BIN_DIR)
	@$(call build-module-cmd,$(1))

up-$(1): build-$(1)
	@if [ -n "$(MODULE_$(1)_PROFILES)" ]; then docker compose --profile $(MODULE_$(1)_PROFILES) up -d; fi
	@$(call start-module-cmd,$(1))
	@$(call echo-module-url-cmd,$(1))

down-$(1):
	@$(call down-module-cmd,$(1))
endef

$(foreach m,$(MODULES),$(eval $(call module-targets,$(m))))

build:
	@mkdir -p $(BIN_DIR)
	@$(foreach m,$(SELECTED_MODULES),$(call build-module-cmd,$(m)))

tidy:
	go mod tidy

up: build
ifneq ($(UP_PROFILES),)
	@if [ "$(UP_PROFILES)" = "all" ]; then \
		docker compose --profile $(COMPOSE_PROFILES) up -d; \
	else \
		docker compose $(foreach p,$(UP_PROFILES),--profile $(p)) up -d; \
	fi
endif
	@$(foreach m,$(SELECTED_MODULES),$(call start-module-cmd,$(m)))
	@$(foreach m,$(SELECTED_MODULES),$(call echo-module-url-cmd,$(m)))

down:
	@$(foreach m,$(SELECTED_MODULES),$(call down-module-cmd,$(m)))
	@if [ "$(SELECTED_MODULES)" = "$(MODULES)" ]; then docker compose down; fi
