.PHONY: dev dev-frontend dev-backend build build-frontend build-backend embed-and-compile test test-frontend ci run docker clean

# Development — run each in a separate terminal.
dev-backend:
	ACCESS_KEY=dev-key go run ./cmd/gateway/

dev-frontend:
	cd frontend && npm run dev

# Build
build: build-frontend build-backend

build-frontend:
	cd frontend && npm ci && npm run build

# Copy built SPA into embed dir and compile the gateway binary.
embed-and-compile:
	mkdir -p cmd/gateway/frontend_dist
	cp -r frontend/build/* cmd/gateway/frontend_dist/
	go build -o bin/gateway ./cmd/gateway/

build-backend: build-frontend
	$(MAKE) embed-and-compile

# Full validation: one `npm ci`, checks, vite build, Go tests, embedded binary (no second `npm ci`).
ci:
	cd frontend && npm ci && npm run check && npm run build
	go test ./...
	$(MAKE) embed-and-compile

# Run the binary produced by `make build` (set ACCESS_KEY in your environment).
run: build
	./bin/gateway

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
