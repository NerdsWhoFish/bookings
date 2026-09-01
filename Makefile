BIN := bookings

.PHONY: build test lint dev web-build

build: web-build
	GOWORK=off go build -o bin/$(BIN) ./cmd/bookings

web-build:
	npm --prefix web ci
	npm --prefix web run build

test:
	GOWORK=off go test ./...
	npm --prefix web test

lint:
	GOWORK=off go vet ./...
	npm --prefix web run lint
	tofu fmt -check -recursive infra
	tofu -chdir=infra/examples/basic init -backend=false
	tofu -chdir=infra/examples/basic validate

dev:
	GOWORK=off BOOKINGS_DEV_MODE=true go run ./cmd/bookings
