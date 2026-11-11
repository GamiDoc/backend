#!/usr/bin/env bash
# Runs the backend locally with the variables of .env.dev (git-ignored; start from
# .env.dev.example). Extra arguments go to the binary, e.g. `scripts/run-dev.sh doctor`.
set -euo pipefail
cd "${BASH_SOURCE[0]%/*}/.."

if [[ ! -f .env.dev ]]; then
    echo "Missing .env.dev: copy .env.dev.example to .env.dev and fill it in." >&2
    exit 1
fi

set -a
# shellcheck disable=SC1091
source <(sed 's/\r$//' .env.dev)
set +a

if [[ "${AI_PROVIDER:-noop}" != "noop" && -z "${AI_API_KEY:-}" ]]; then
    echo "AI_PROVIDER=${AI_PROVIDER} but AI_API_KEY is empty in .env.dev." >&2
    exit 1
fi

exec go run ./cmd/gamidoc-backend "${@:-serve}"
