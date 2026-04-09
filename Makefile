.PHONY: dev dev-frontend dev-backend build build-frontend build-backend test test-frontend docker clean

# Development — run each in a separate terminal.
dev-backend:
	ACCESS_KEY=dev-key go run ./cmd/gateway/

dev-frontend:
	cd frontend && npm run dev

# Build
build: build-frontend build-backend

build-frontend:
	cd frontend && npm ci && npm run build

build-backend: build-frontend
	mkdir -p cmd/gateway/frontend_dist
	cp -r frontend/build/* cmd/gateway/frontend_dist/
	go build -o bin/gateway ./cmd/gateway/

# Test
test:
	go test ./...

test-frontend:
	cd frontend && npm run check

# Docker
docker:
	docker build -t llmate .

# Clean
clean:
	rm -rf bin/ cmd/gateway/frontend_dist/ frontend/build/
