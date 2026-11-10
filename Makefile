GO = docker run --rm -v "$(CURDIR)":/src -w /src golang:1.27.3-bookworm
.PHONY: demo dbqueue test
demo: ## run every demo (no database needed)
	$(GO) sh -c 'go run ./cmd/hashdemo && go run ./cmd/filterdemo && go run ./cmd/timerdemo && go run ./cmd/partdemo && go run ./cmd/waldemo'
test:
	$(GO) go test -race -count=1 ./...
dbqueue: ## delayed jobs in PostgreSQL (starts a database)
	docker compose up -d --wait postgres
	docker run --rm --network host -v "$(CURDIR)":/src -w /src -e DATABASE_URL=postgres://lab:lab@127.0.0.1:55434/lab?sslmode=disable golang:1.27.3-bookworm go run ./cmd/dbqueue
