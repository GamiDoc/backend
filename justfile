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
    go run ./cmd/gamidoc-backend serve

build:
    go build ./cmd/gamidoc-backend

migrate-up:
    go run ./cmd/gamidoc-backend migrate up

migrate-status:
    go run ./cmd/gamidoc-backend migrate status

doctor:
    go run ./cmd/gamidoc-backend doctor

version:
    go run ./cmd/gamidoc-backend version

db-migrate:
    just migrate-up

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
