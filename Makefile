APP_NAME := monic
VERSION ?= dev

HOST := $(shell grep '^HOST=' .env | cut -d '=' -f 2)

# Build and test
.PHONY: build
build:
	@echo "Building $(APP_NAME)..."
	@go build -mod=vendor -ldflags="-X main.version=$(VERSION)" -o $(APP_NAME) main.go

.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):latest .

# Install — copy configs to remote host
.PHONY: install
install:
	@echo "Installing $(APP_NAME) on $(HOST)..."
	-ssh root@$(HOST) "mkdir -p /opt/$(APP_NAME)"
	scp ./.env root@$(HOST):/opt/$(APP_NAME)/.env
	scp ./docker-compose.yml root@$(HOST):/opt/$(APP_NAME)/docker-compose.yml
	@echo "Install complete. Run 'make deploy' to start."

# Deploy — pull latest image and restart
.PHONY: deploy
deploy:
	@echo "Deploying $(APP_NAME) to $(HOST)..."
	ssh root@$(HOST) "docker pull ghcr.io/mikhail-angelov/$(APP_NAME):latest"
	-ssh root@$(HOST) "cd /opt/$(APP_NAME) && docker compose down"
	ssh root@$(HOST) "cd /opt/$(APP_NAME) && docker compose up -d"
	@echo "Deploy complete."

# Help
.PHONY: help
help:
	@echo "Monic Monitoring — available targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Quick start:"
	@echo "  1. Edit .env — set HOST and any MONIC_* vars"
	@echo "  2. make install   — copy configs to server"
	@echo "  3. make deploy    — pull image and start monic"
