# msgpack (Quad4 fork)

This repository is a **fork** of [github.com/vmihailenco/msgpack](https://github.com/vmihailenco/msgpack) (v5 API), **maintained by Quad4** at `git.quad4.io/Go-Libs/msgpack`.

The wire format and public API are unchanged: every `Marshal` / `Unmarshal` / `Encoder` / `Decoder` call signature, struct tag, and option from upstream `v5.4.1` continues to work. This fork adds security, correctness, and performance fixes; it does not introduce breaking changes.

## Drop-in migration from `github.com/vmihailenco/msgpack/v5`

1. Replace the import path:

   ```go
   // before
   import "github.com/vmihailenco/msgpack/v5"

   // after
   import "git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack"
   ```

   The package is still imported as `msgpack`, so call sites do not need to change.

2. Pull the module:

   ```bash
   go get git.quad4.io/Go-Libs/msgpack/v5@latest
   ```

3. (Optional) For users of the App Engine helpers:

   ```go
   // before
   import "github.com/vmihailenco/msgpack/v5/msgpappengine"

   // after
   import "git.quad4.io/Go-Libs/msgpack/extra/msgpappengine"
   ```

   This is a separate Go module; install with `go get git.quad4.io/Go-Libs/msgpack/extra/msgpappengine@latest`.

The wire codes (subpackage `msgpcode`) move from `github.com/vmihailenco/msgpack/v5/msgpcode` to `git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack/msgpcode`. Constants are unchanged.

## What this fork fixes vs. upstream `v5.4.1`

The upstream module has been effectively unmaintained for several years. This fork addresses, without changing the API:

### Security

- **OOM via forged length prefixes.** Four decode paths (`bytes`, `bytesPtr`, `decodeSlice`, `decodeSliceValue`, `DecodeMap`, `DecodeUntypedMap`) trusted the on-wire length and allocated up front, so a single hostile `bin32` / `str32` / `array32` / `map32` header could request a multi-gigabyte allocation before the underlying short read failed. All four now clamp the initial allocation to the documented per-decoder limit and grow incrementally as real bytes arrive. The `disableAllocLimitFlag` toggle (`Decoder.DisableAllocLimit(true)`) preserves the legacy unbounded behaviour for callers that want it.
- **`disableAllocLimitFlag` was a no-op in `decodeSliceValue`.** Upstream compared the bit flag against the literal `1`, but the flag is `1 << 1 == 2`, so the limit was never applied along the typed-slice path. The comparison is now `!= 0`.
- **Goroutine and memory leak in the per-type preallocator.** The upstream `cachedValue` spawned one perpetual goroutine for every distinct `reflect.Type` ever decoded into, plus a 256-slot buffered channel each, and held a global `sync.RWMutex` on the hot decode path. A long-running process that ever decoded into N distinct Go types retained N goroutines forever. The preallocator is now backed by a `sync.Map` of `*sync.Pool`; idle entries are reclaimed by the GC, the global mutex is gone from the lookup path, and goroutine count stays flat under churn. Verified by `TestNoGoroutineLeakOnDistinctTypes` and `TestConcurrentDistinctTypesPreallocate` in `pkg/msgpack/leak_test.go`.

### Correctness

- **`invalid code=cb decoding int64` on JSON-sourced data.** `(*Decoder).int` and `(*Decoder).uint` now accept `msgpcode.Float` / `msgpcode.Double` payloads when the destination is a Go integer, provided the value is finite, integer-valued, and in range. This unblocks the common JSON -> `map[string]any` -> msgpack -> struct round-trip (`encoding/json` decodes JSON numbers into `float64` regardless of declared field type). NaN, infinities, fractional values, negative values into `uint64`, and out-of-range magnitudes still error rather than silently truncating.
- **Pooled `*bytes.Reader` does not retain caller data.** `Marshal` / `Unmarshal` reuse internal buffers via `sync.Pool`; the wrappers are reset to `nil` before being returned to the pool so they cannot leak the previous caller's slice into a subsequent call. Verified by `TestPoolDoesNotRetainCallerData` and `TestInvariantPoolDoesNotAlias`.

### Performance (vs. upstream `v5.4.1`, geomean over 5 x 2s `benchstat` runs)

- `Unmarshal`: pooled `*bytes.Reader` wrapper. `BenchmarkStructUnmarshal` -50% B/op (96 -> 48), -1 alloc/op, -5.16% time. `BenchmarkStructUnmarshalPartially` -75% B/op (64 -> 16), -1 alloc/op, -6.68% time.
- `Marshal`: pre-grows the encode buffer to 64 bytes, skipping the first one or two backing-array doublings for typical small payloads. The returned slice still owns its backing array; aliasing semantics are preserved.
- `byteWriter.WriteByte`: writes through a 1-byte field on the wrapper struct instead of allocating a fresh `[]byte{c}` per call. `BenchmarkDiscard` -100% B/op, -100% allocs/op.
- Pooled `*Encoder` and `*Decoder` (`GetEncoder` / `PutEncoder`, `GetDecoder` / `PutDecoder`) work as before; reuse benchmarks added in `pkg/msgpack/bench_test.go`.

## Install

```bash
go get git.quad4.io/Go-Libs/msgpack/v5@latest
```

```go
import "git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack"
```

The module path is `git.quad4.io/Go-Libs/msgpack/v5` (the `/v5` suffix matches the major version). Source lives under `pkg/msgpack/`; subpackage `msgpcode` is at `pkg/msgpack/msgpcode`.

## Features

- Primitives, arrays, maps, structs, `time.Time`, and `interface{}`.
- App Engine `*datastore.Key` and `datastore.Cursor` via `extra/msgpappengine` (optional module).
- `CustomEncoder` / `CustomDecoder` for custom encoding.
- Extensions, struct tags (`msgpack:"..."`), omitempty, sorted map keys, array-encoded structs, and `Decoder.Query`-style path queries.

## Layout

| Path | Purpose |
|------|---------|
| `pkg/msgpack` | Public API (`Marshal`, `Encoder`, `Decoder`, etc.) |
| `pkg/msgpack/msgpcode` | Wire format opcode constants |
| `extra/msgpappengine` | Optional Google App Engine helpers (separate `go.mod`) |
| `go.work` | Workspace: root module + `extra/msgpappengine` for local `go test ./...` |
| `scripts/ci` | Local CI parity with Gitea (`test-all.sh`, `scan-all.sh`, `setup-go.sh`, ...) |
| `.gitea/workflows` | CI (`ci.yml`) and security scan (`scan.yml`) |

## Testing

- **Unit tests** in `pkg/msgpack/*_test.go` cover the decoder/encoder surface, struct round-trips, time, ext, intern, and query paths.
- **Property-based tests** via [pbt](https://git.quad4.io/Go-Libs/pbt) (`git.quad4.io/Go-Libs/pbt/pkg/pbt`) in `pbt_test.go` (roundtrip properties for `[]byte`, `string`, `[]int`, `[]string`, `map[string]int`, `map[string]string`).
- **Fuzz targets** in `fuzz_test.go`: `FuzzMarshalUnmarshalRoundtrip`, `FuzzUnmarshalArbitrary`, `FuzzDecoderQuery`, `FuzzDecodeIntoStruct`, `FuzzDecodeExtHeader`, `FuzzDecodeTime`, `FuzzDecodeInternedString`, `FuzzDecodeSkip`, `FuzzDecodeMulti`. Regression corpora for the allocation-limit fixes are committed under `pkg/msgpack/testdata/fuzz/`.
- **Stress tests** in `stress_test.go`: concurrent `Marshal` / `Unmarshal` under `-race`, large byte slice / string round-trips, deeply nested map and slice round-trips, repeated reuse of pooled encoder/decoder.
- **Invariant tests** in `invariant_test.go`: `Unmarshal(nil)` / empty input never panics; `Marshal(nil)` is a single `msgpcode.Nil` byte; bit-exact round-trip of `int64` min, `uint64` max, `NaN`, `-Inf`; pool aliasing checks.
- **Leak tests** in `leak_test.go`: per-type preallocator must not retain goroutines or values across decoder churn.

Run `go test -race ./...` for the full suite; `make` runs `go vet` plus tests.

## License

BSD 2-clause; see [LICENSE](LICENSE). Original copyright remains with the vmihailenco authors; fork maintenance is attributed in this README.
