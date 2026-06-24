.PHONY: mod build test tag clean proto \
	lab-build lab-up lab-down \
	$(foreach m,kv config lb queue dag ui,lab-build-$(m) lab-up-$(m) lab-down-$(m)) \
	lab-build-dag-http lab-up-dag-http lab-down-dag-http

# Module paths
ROOT_MODULE := .
SUB_MODULES := \
	pkg/storage/kv/redis \
	pkg/storage/kv/boltdb \
	pkg/storage/kv/aerospike \
	pkg/rpc/governance/config/consul \
	pkg/messaging/queue/rabbitmq \
	pkg/messaging/queue/nats \
	pkg/messaging/queue/natsjetstream \
	pkg/messaging/queue/kafka

LAB_MODULE := lab

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFLAGS := -v

.PHONY: all
all: mod build

# Generate protobuf code
.PHONY: proto
proto:
	@echo ">>> Generating protobuf code"
	buf generate

# Run go mod tidy for all modules
.PHONY: mod
mod:
	@echo ">>> Tidying root module"
	$(GOMOD) tidy
	@for module in $(SUB_MODULES); do \
		echo ">>> Tidying $$module"; \
		(cd $$module && $(GOMOD) tidy); \
	done

# Build all modules
.PHONY: build
build:
	@echo ">>> Building root module"
	$(GOBUILD) $(GOFLAGS) ./...
	@for module in $(SUB_MODULES); do \
		echo ">>> Building $$module"; \
		(cd $$module && $(GOBUILD) $(GOFLAGS) ./...); \
	done

# Run tests for all modules (lab excluded)
.PHONY: test
test:
	@echo ">>> Testing root module"
	$(GOTEST) $(GOFLAGS) ./...
	@for module in $(SUB_MODULES); do \
		echo ">>> Testing $$module"; \
		(cd $$module && $(GOTEST) $(GOFLAGS) ./...); \
	done

# Lab targets（LAB_MODULES 可选：all | kv,dag | "kv dag"）
LAB_MAKE_OPTS := $(if $(LAB_MODULES),LAB_MODULES="$(LAB_MODULES)",)

define lab-forward-targets
.PHONY: lab-build-$(1) lab-up-$(1) lab-down-$(1)
lab-build-$(1):
	$(MAKE) -C $(LAB_MODULE) build-$(1)

lab-up-$(1):
	$(MAKE) -C $(LAB_MODULE) up-$(1)

lab-down-$(1):
	$(MAKE) -C $(LAB_MODULE) down-$(1)
endef

$(foreach m,kv config lb queue dag ui,$(eval $(call lab-forward-targets,$(m))))

# lab-dag-http 按域转发（lab 模块注册键为 daghttp，端口 8086）。
lab-build-dag-http:
	$(MAKE) -C $(LAB_MODULE) build-daghttp

lab-up-dag-http:
	$(MAKE) -C $(LAB_MODULE) up-daghttp

lab-down-dag-http:
	$(MAKE) -C $(LAB_MODULE) down-daghttp

lab-build:
	$(MAKE) -C $(LAB_MODULE) build $(LAB_MAKE_OPTS)

lab-up:
	$(MAKE) -C $(LAB_MODULE) up $(LAB_MAKE_OPTS)

lab-down:
	$(MAKE) -C $(LAB_MODULE) down $(LAB_MAKE_OPTS)

# Create version tags for root module and all submodules
# Usage: make tag V=v0.2.0          (local only)
#        make tag V=v0.2.0 PUSH=1   (create and push)
.PHONY: tag
tag:
ifndef V
	$(error Usage: make tag V=vX.Y.Z [PUSH=1])
endif
	@ARGS="$(V)"; \
	if [ "$(PUSH)" = "1" ]; then ARGS="$$ARGS --push"; fi; \
	./scripts/tag.sh $$ARGS

# Clean build artifacts
.PHONY: clean
clean:
	@echo ">>> Cleaning"
	@rm -rf bin/
	@find . -name "*.test" -delete
	@find . -name "*.out" -delete

# Help
.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  proto         Generate protobuf code from api/dag/v1"
	@echo "  mod           Run go mod tidy for all modules"
	@echo "  build         Build all modules"
	@echo "  test          Run tests for all modules (excludes lab)"
	@echo "  lab-build     Build lab CLI binaries (all domains)"
	@echo "  lab-up        Start lab compose + serve processes (all domains)"
	@echo "  lab-down      Stop lab processes and compose (all domains)"
	@echo "  lab-build-<m> Build one domain (kv|config|lb|queue|dag|ui)"
	@echo "  lab-up-<m>    Start one domain (e.g. lab-up-dag, lab-up-dag-http)"
	@echo "  lab-down-<m>  Stop one domain process only"
	@echo "  LAB_MODULES   Subset for lab-build/up/down (e.g. LAB_MODULES=kv,dag)"
	@echo "  tag           Create version tags (V=vX.Y.Z, PUSH=1 to push)"
	@echo "  clean         Clean build artifacts"
	@echo "  help          Show this help message"
