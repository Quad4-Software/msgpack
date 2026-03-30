#!/bin/sh
# Run the same checks as CI: vet and tests (with race). Run from repo root or via this path.
set -eu
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
go vet ./...
go test -count=1 -race ./...
