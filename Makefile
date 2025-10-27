.PHONY: all fmt lint test coverage benchmark deps air run api-docs release clean help

all: fmt lint test benchmark ## Run all tests and checks

fmt: ## Format the code
	golangci-lint fmt

lint: ## Lint the code
	golangci-lint run --timeout=5m

test: ## Run tests
	go test -race -shuffle=on -count=1 -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...

coverage: test ## Generate coverage
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

benchmark: ## Run benchmarks
	go test -run=^$$ -bench=. -benchmem ./... | tee benchmark.txt

deps: ## Install dependencies
	go mod download
	go install github.com/cosmtrek/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest

air: ## Run Air live-reload
	air -c .air.toml

run: ## Run the app in Docker
	docker compose up --build

api-docs: ## Generate API docs
	@if ! swag fmt -g ./main.go; then \
		echo "Error: Failed to format API docs"; \
		exit 1; \
	fi
	@if ! swag init --parseDependency -g ./main.go -o ./api; then \
		echo "Error: Failed to generate API docs"; \
		exit 1; \
	fi

release: ## Create release
	goreleaser release --snapshot --clean

clean: ## Remove build artifacts
	rm -f coverage.* benchmark.txt
	rm -rf dist

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
