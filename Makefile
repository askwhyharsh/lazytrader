

build: ## Build the application
	@echo "Building application..."
	@echo "Build complete! Binary: bin/server"
	go build -o bin/server cmd/main.go

run: ## Run the application
	@echo "Starting server..."
	go run cmd/main.go

test: ## Run unit tests
	@echo "Running tests..."
	go test -v -race -timeout 30s ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Clean complete!"

redis-up: ## Start Redis in Docker
	@echo "Starting Redis..."
	docker run -d --name lazytrader-redis -p 6379:6379 redis:7-alpine
	@echo "Redis started on port 6379"

redis-down: ## Stop Redis Docker container
	@echo "Stopping Redis..."
	docker stop lazytrader-redis || true
	docker rm lazytrader-redis || true
	@echo "Redis stopped"