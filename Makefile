.PHONY: build run clean docker-build docker-run

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
