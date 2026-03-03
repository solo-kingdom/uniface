.PHONY: mod build test clean

# Module paths
ROOT_MODULE := .
SUB_MODULES := pkg/storage/kv/redis

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFLAGS := -v

.PHONY: all
all: mod build

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

# Run tests for all modules
.PHONY: test
test:
	@echo ">>> Testing root module"
	$(GOTEST) $(GOFLAGS) ./...
	@for module in $(SUB_MODULES); do \
		echo ">>> Testing $$module"; \
		(cd $$module && $(GOTEST) $(GOFLAGS) ./...); \
	done

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
	@echo "  mod     Run go mod tidy for all modules"
	@echo "  build   Build all modules"
	@echo "  test    Run tests for all modules"
	@echo "  clean   Clean build artifacts"
	@echo "  help    Show this help message"
