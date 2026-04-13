set shell := ["bash", "-lc"]

default:
    just --list

init:
    just mod-tidy

mod-tidy:
    go mod tidy

fmt:
    go fmt ./...

fmt-check:
    test -z "$(gofmt -l $(find . -type f -name '*.go' -not -path './vendor/*'))"

lint:
    go vet ./...

lint-check:
    go vet ./...

test:
    go test ./...

run:
    go run ./cmd/api

build:
    go build ./cmd/api

db-migrate:
    set -a
    source .env.dev
    set +a
    for file in migrations/*.sql; do \
      psql "postgresql://$POSTGRES_USER:$POSTGRES_PASSWORD@$POSTGRES_HOST:$POSTGRES_PORT/$POSTGRES_DB?sslmode=disable" -v ON_ERROR_STOP=1 -f "$file"; \
    done

ci:
    just mod-tidy && \
    just fmt-check && \
    just lint-check && \
    just test

docs-gen:
    python3 scripts/gen_docs.py

docs-clean:
    rm -f docs/reference/*.md

docs-refresh:
    just docs-clean && \
    just docs-gen
