BACKEND_DIR := backend
FRONTEND_DIR := frontend
BACKEND_BIN_DIR := $(BACKEND_DIR)/bin

ifeq ($(OS),Windows_NT)
BACKEND_BIN_NAME := minedock-backend.exe
define MAKE_DIR
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force '$(1)' | Out-Null"
endef
define REMOVE_DIR
	powershell -NoProfile -Command "if (Test-Path '$(1)') { Remove-Item -Recurse -Force '$(1)' }"
endef
else
BACKEND_BIN_NAME := minedock-backend
define MAKE_DIR
	mkdir -p $(1)
endef
define REMOVE_DIR
	rm -rf $(1)
endef
endif

.PHONY: help dev dev-backend dev-frontend build build-backend build-frontend clean install-frontend

help:
	@echo "Available targets:"
	@echo "  make dev               # start backend and frontend dev servers"
	@echo "  make build             # build backend binary and frontend assets"
	@echo "  make clean             # remove backend and frontend build outputs"
	@echo "  make install-frontend  # install frontend dependencies"

dev:
	@$(MAKE) -j2 dev-backend dev-frontend

dev-backend:
	cd $(BACKEND_DIR) && go run main.go

dev-frontend:
	cd $(FRONTEND_DIR) && npm run dev

build: build-backend build-frontend

build-backend:
	$(call MAKE_DIR,$(BACKEND_BIN_DIR))
	cd $(BACKEND_DIR) && go build -o bin/$(BACKEND_BIN_NAME) .

build-frontend:
	cd $(FRONTEND_DIR) && npm run build

clean:
	$(call REMOVE_DIR,$(BACKEND_BIN_DIR))
	$(call REMOVE_DIR,$(FRONTEND_DIR)/dist)

install-frontend:
	cd $(FRONTEND_DIR) && npm install