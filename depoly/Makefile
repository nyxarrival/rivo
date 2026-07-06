ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
DIST_DIR ?= $(ROOT_DIR)/dist
EMBED_PANEL_DIR ?= $(ROOT_DIR)/internal/master/web/dist
MASTER_ENV ?= $(ROOT_DIR)/depoly/master/.env
AGENT_ENV ?= $(ROOT_DIR)/depoly/agent/.env
GO ?= go
PNPM ?= pnpm
DOCKER_COMPOSE ?= docker compose
MASTER_DB_DRIVER := $(shell awk -F= '/^RIVO_DATABASE_DRIVER=/{print tolower($$2)}' "$(MASTER_ENV)" 2>/dev/null)
MASTER_DB_PROFILE := $(if $(filter mysql,$(MASTER_DB_DRIVER)),--profile mysql,)
MASTER_DB_SERVICES := $(if $(filter mysql,$(MASTER_DB_DRIVER)),master mysql,master)

VERSION ?= $(shell cd "$(ROOT_DIR)" && git describe --tags --always --dirty 2>/dev/null || echo dev)
CGO_ENABLED ?= 1

MASTER_PKG := ./cmd/master
AGENT_PKG := ./cmd/agent
MASTER_BIN := rivo-master
AGENT_BIN := rivo-agent

GO_BUILD_FLAGS := -trimpath
MASTER_LDFLAGS := -s -w
AGENT_LDFLAGS := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "Rivo deploy targets"
	@echo ""
	@echo "Binary:"
	@echo "  make -f depoly/Makefile panel-build"
	@echo "  make -f depoly/Makefile panel-embed"
	@echo "  make -f depoly/Makefile master"
	@echo "  make -f depoly/Makefile agent"
	@echo "  make -f depoly/Makefile build"
	@echo ""
	@echo "Docker:"
	@echo "  cp depoly/master/.env.example depoly/master/.env"
	@echo "  cp depoly/agent/.env.example depoly/agent/.env"
	@echo "  make -f depoly/Makefile docker-master"
	@echo "  make -f depoly/Makefile docker-agent"
	@echo "  make -f depoly/Makefile compose-master-up   # starts mysql too when RIVO_DATABASE_DRIVER=mysql"
	@echo "  make -f depoly/Makefile compose-mysql-up    # explicit mysql profile"
	@echo "  make -f depoly/Makefile compose-agent-up"
	@echo "  make -f depoly/Makefile compose-master-down"
	@echo "  make -f depoly/Makefile compose-agent-down"

.PHONY: panel-build
panel-build:
	@echo "==> build panel"
	@cd "$(ROOT_DIR)/panel" && $(PNPM) install --frozen-lockfile
	@cd "$(ROOT_DIR)/panel" && $(PNPM) build

.PHONY: panel-embed
panel-embed: panel-build
	@echo "==> sync panel dist into Go embed directory"
	@mkdir -p "$(EMBED_PANEL_DIR)"
	@rm -rf "$(EMBED_PANEL_DIR)"/*
	@cp -R "$(ROOT_DIR)/panel/dist/." "$(EMBED_PANEL_DIR)/"

.PHONY: master
master: panel-embed
	@mkdir -p "$(DIST_DIR)"
	@echo "==> build master version=$(VERSION)"
	@cd "$(ROOT_DIR)" && CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GO_BUILD_FLAGS) -ldflags "$(MASTER_LDFLAGS)" -o "$(DIST_DIR)/$(MASTER_BIN)" $(MASTER_PKG)

.PHONY: agent
agent:
	@mkdir -p "$(DIST_DIR)"
	@echo "==> build agent version=$(VERSION)"
	@cd "$(ROOT_DIR)" && CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GO_BUILD_FLAGS) -ldflags "$(AGENT_LDFLAGS)" -o "$(DIST_DIR)/$(AGENT_BIN)" $(AGENT_PKG)

.PHONY: build
build: master agent

.PHONY: ensure-master-env
ensure-master-env:
	@test -f "$(MASTER_ENV)" || (echo "missing depoly/master/.env, copy depoly/master/.env.example first"; exit 1)

.PHONY: ensure-agent-env
ensure-agent-env:
	@test -f "$(AGENT_ENV)" || (echo "missing depoly/agent/.env, copy depoly/agent/.env.example first"; exit 1)

.PHONY: docker-master
docker-master: ensure-master-env
	@cd "$(ROOT_DIR)" && $(DOCKER_COMPOSE) --env-file "$(MASTER_ENV)" -f depoly/master/compose.yml $(MASTER_DB_PROFILE) build master

.PHONY: docker-agent
docker-agent: ensure-agent-env
	@cd "$(ROOT_DIR)" && $(DOCKER_COMPOSE) --env-file "$(AGENT_ENV)" -f depoly/agent/compose.yml build agent

.PHONY: docker
docker: docker-master docker-agent

.PHONY: compose-master-up
compose-master-up: ensure-master-env
	@cd "$(ROOT_DIR)" && $(DOCKER_COMPOSE) --env-file "$(MASTER_ENV)" -f depoly/master/compose.yml $(MASTER_DB_PROFILE) up -d --build $(MASTER_DB_SERVICES)

.PHONY: compose-mysql-up
compose-mysql-up: ensure-master-env
	@cd "$(ROOT_DIR)" && $(DOCKER_COMPOSE) --env-file "$(MASTER_ENV)" -f depoly/master/compose.yml --profile mysql up -d --build

.PHONY: compose-agent-up
compose-agent-up: ensure-agent-env
	@cd "$(ROOT_DIR)" && $(DOCKER_COMPOSE) --env-file "$(AGENT_ENV)" -f depoly/agent/compose.yml up -d --build

.PHONY: compose-master-down
compose-master-down: ensure-master-env
	@cd "$(ROOT_DIR)" && $(DOCKER_COMPOSE) --env-file "$(MASTER_ENV)" -f depoly/master/compose.yml $(MASTER_DB_PROFILE) down

.PHONY: compose-agent-down
compose-agent-down: ensure-agent-env
	@cd "$(ROOT_DIR)" && $(DOCKER_COMPOSE) --env-file "$(AGENT_ENV)" -f depoly/agent/compose.yml down

.PHONY: clean
clean:
	@rm -rf "$(DIST_DIR)" "$(ROOT_DIR)/panel/dist"
