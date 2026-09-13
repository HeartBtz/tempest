.PHONY: all build run clean frontend backend dev install

VERSION := 0.1.0
BINARY := tempest
BUILD_DIR := build

all: frontend backend

backend:
	@echo "⚡ Building Tempest backend..."
	CGO_ENABLED=1 go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY) ./cmd/tempest

frontend:
	@echo "⚡ Building Tempest frontend..."
	cd web && npm ci && npm run build

run: backend
	@echo "⚡ Starting Tempest..."
	./$(BUILD_DIR)/$(BINARY)

dev-backend:
	@echo "⚡ Starting Tempest backend (dev)..."
	CGO_ENABLED=1 go run ./cmd/tempest

dev-frontend:
	@echo "⚡ Starting Tempest frontend (dev)..."
	cd web && npm run dev

clean:
	rm -rf $(BUILD_DIR)
	rm -rf web/dist
	rm -rf web/node_modules

docker:
	docker build -t tempest:$(VERSION) .

test:
	go test ./...

install: all
	@echo "⚡ Running install script..."
	./install.sh --standalone

help:
	@echo "Tempest ⚡ - BitTorrent Announce Testing Dashboard"
	@echo ""
	@echo "Commands:"
	@echo "  make all          - Build frontend and backend"
	@echo "  make backend      - Build Go backend"
	@echo "  make frontend     - Build React frontend"
	@echo "  make run          - Build and run"
	@echo "  make dev-backend  - Run backend in dev mode"
	@echo "  make dev-frontend - Run frontend in dev mode"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make docker       - Build Docker image"
	@echo "  make test         - Run tests"
	@echo "  make install      - Build & install (standalone)"
