.PHONY: build test vet fmt up down logs demo tidy

build: ## Compile all service binaries into ./bin
	@mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/locationworker ./cmd/locationworker
	go build -o bin/tripworker ./cmd/tripworker
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/web ./cmd/web

test: ## Run unit tests (no infra required)
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

up: ## Build images and start the full stack
	docker compose up --build -d

down: ## Stop the stack and remove volumes
	docker compose down -v

logs:
	docker compose logs -f api locationworker tripworker gateway

demo: ## Run the end-to-end demo against a running stack
	./scripts/demo.sh
