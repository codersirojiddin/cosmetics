.PHONY: run test migrate seed vet
run:
	go run ./cmd/server
migrate:
	go run ./cmd/migrate
seed:
	go run ./cmd/seed
vet:
	go vet ./... && test -z "$$(gofmt -l .)"
test:
	go test -race ./...
