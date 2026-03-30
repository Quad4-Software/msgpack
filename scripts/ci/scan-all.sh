#!/bin/sh
# Security scans: gosec and govulncheck (same as .gitea/workflows/scan.yml). Optional Trivy if installed.
set -eu
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
export PATH="$(go env GOPATH)/bin:${PATH}"
go install github.com/securego/gosec/v2/cmd/gosec@v2.24.5
gosec ./...
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
govulncheck ./...
if command -v trivy >/dev/null 2>&1; then
	trivy fs --exit-code 1 --severity HIGH,CRITICAL .
fi
