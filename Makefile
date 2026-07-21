.PHONY: build run clean test coverage e2e e2e-setup \
	docker-build docker-run docker-stop logs

# Build the Go binary
build:
	go build -o redchef .

# Run locally with default admin credentials
run: build
	./redchef

# Clean build artifacts
clean:
	rm -f redchef
	rm -f redchef.db
	rm -rf uploads/*
	rm -rf node_modules
	rm -rf playwright-report

# Run all Go tests with verbose output
test:
	go test -v ./...

# Run Go tests with coverage report
coverage:
	go test -v -coverprofile=coverage.out -covermode=count ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Install Playwright dependencies
e2e-setup:
	npm ci
	npx playwright install chromium

# Build Go binary and run Playwright E2E tests
e2e: build
	go build -o /tmp/redchef-e2e ./...
	npx playwright test

# Build Docker image
docker-build:
	docker compose build

# Run with Docker Compose
docker-run:
	docker compose up

# Stop Docker Compose
docker-stop:
	docker compose down

# View logs
logs:
	docker compose logs -f
