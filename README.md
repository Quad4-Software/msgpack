# msgpack (Quad4 fork)

This repository is a **fork** of [github.com/vmihailenco/msgpack](https://github.com/vmihailenco/msgpack) (v5 API), **maintained by Quad4** at `git.quad4.io/Go-Libs/msgpack`. The upstream project is largely unmaintained; this fork updates the Go toolchain, dependencies, layout, and CI.

Import the library as:

```go
import "git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack"
```

Install:

```bash
go get git.quad4.io/Go-Libs/msgpack/v5@latest
```

The module path is `git.quad4.io/Go-Libs/msgpack/v5` (the `/v5` suffix matches the major version). Source lives under `pkg/msgpack/`; subpackage `msgpcode` is at `pkg/msgpack/msgpcode`.

## Features

- Primitives, arrays, maps, structs, `time.Time`, and `interface{}`.
- App Engine `*datastore.Key` and `datastore.Cursor` via `extra/msgpappengine` (optional module).
- `CustomEncoder` / `CustomDecoder` for custom encoding.
- Extensions, struct tags (`msgpack:"..."`), omitempty, sorted map keys, array-encoded structs, and `Decoder.Query`-style path queries.

Upstream documentation and background: [msgpack.uptrace.dev](https://msgpack.uptrace.dev) and the [original README history](https://github.com/vmihailenco/msgpack).

## Layout

| Path | Purpose |
|------|---------|
| `pkg/msgpack` | Public API (`Marshal`, `Encoder`, `Decoder`, etc.) |
| `pkg/msgpack/msgpcode` | Wire format opcode constants |
| `extra/msgpappengine` | Optional Google App Engine helpers (separate `go.mod`) |
| `go.work` | Workspace: root module + `extra/msgpappengine` for local `go test ./...` |
| `scripts/ci` | Local CI parity with Gitea (`test-all.sh`, `scan-all.sh`, `setup-go.sh`, …) |
| `.gitea/workflows` | CI (`ci.yml`) and security scan (`scan.yml`) |

## Testing

- **Property-based tests** use [pbt](https://git.quad4.io/Go-Libs/pbt) (`git.quad4.io/Go-Libs/pbt/pkg/pbt`) in `pbt_test.go` (roundtrip properties for `[]byte`, `string`, and `map[string]int`).

## License

BSD 2-clause; see [LICENSE](LICENSE). Original copyright remains with the vmihailenco authors; fork maintenance is attributed in this README.
