.PHONY: generate frontend build test vet run
frontend:
	cd frontend && bun install --frozen-lockfile && bun run build
	cp -R frontend/dist/. internal/web/assets/
	test -f internal/web/assets/index.html
generate:
	cd frontend && bun install --frozen-lockfile && bun run contracts
build: generate frontend
	go build -o bin/ksi-service ./cmd/ksi-service
test:
	go test ./...
vet:
	go vet ./...
run: generate frontend
	go run ./cmd/ksi-service
