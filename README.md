# msgpack

Fork of [github.com/vmihailenco/msgpack](https://github.com/vmihailenco/msgpack), v5 API, published under the module path `github.com/Quad4-Software/msgpack/v5`.

The wire format and public API match upstream v5.4.1. `Marshal`, `Unmarshal`, `Encoder`, `Decoder`, struct tags, and options keep the same signatures. This fork adds security, correctness, and performance fixes without breaking changes.

## Migrating from upstream

Change the import path. The package name stays `msgpack`, so call sites do not change.

```go
// before
import "github.com/vmihailenco/msgpack/v5"

// after
import "github.com/Quad4-Software/msgpack/v5/pkg/msgpack"
```

App Engine helpers live in a separate module at `github.com/Quad4-Software/msgpack/v5/extra/msgpappengine`. Wire opcode constants live at `github.com/Quad4-Software/msgpack/v5/pkg/msgpack/msgpcode`. Values are unchanged.

## Differences from upstream v5.4.1

Upstream v5.4.1 is the last tagged release on the original module.

### Security

Several decode paths trusted the on-wire length and allocated up front: `bytes`, `bytesPtr`, `decodeSlice`, `decodeSliceValue`, `DecodeMap`, `DecodeUntypedMap`. A hostile bin32, str32, array32, or map32 header could force a multi-gigabyte allocation before the short read failed. Those paths now clamp the first allocation to the per-decoder limit and grow as bytes arrive. `Decoder.DisableAllocLimit(true)` restores the old unbounded behavior.

Decoder recursion has a depth cap through `SetDecodeDepthLimit`, default 10000. Deeply nested payloads fail with an error instead of growing the stack. Length parsing for str32, bin32, array32, map32, and ext32 rejects uint32 values that overflow int on 32-bit builds.

`disableAllocLimitFlag` was compared against the literal 1 in `decodeSliceValue`, but the flag is `1<<1`, so the typed-slice path never applied the limit. The comparison now tests for a nonzero flag.

The upstream `cachedValue` preallocator spawned one permanent goroutine and a 256-slot buffered channel per distinct `reflect.Type`, and held a global `sync.RWMutex` on the decode path. Decoding into N Go types left N goroutines alive forever. The fork uses a `sync.Map` of `sync.Pool` values. Idle entries can be collected, the global mutex is off the lookup path, and the goroutine count stays flat. `TestNoGoroutineLeakOnDistinctTypes` and `TestConcurrentDistinctTypesPreallocate` in `pkg/msgpack/leak_test.go` cover this.

### Correctness

The decoder int and uint paths accept Float and Double wire codes when the destination is a Go integer, provided the value is finite, integer-valued, and in range. That covers the JSON to map to msgpack to struct round trip, since `encoding/json` stores numbers as float64. NaN, infinities, fractions, negative values into uint64, and out-of-range magnitudes still error.

`Marshal` and `Unmarshal` reuse buffers through `sync.Pool`. Wrappers are reset to nil before returning to the pool, so a pooled `bytes.Reader` cannot leak the previous caller's slice. See `TestPoolDoesNotRetainCallerData` and `TestInvariantPoolDoesNotAlias`.

### Performance

Geomean over five 2-second benchstat runs against upstream v5.4.1:

- `Unmarshal` uses a pooled `bytes.Reader`. `BenchmarkStructUnmarshal`: -50% B/op (96 to 48), -1 alloc/op, -5.16% time. `BenchmarkStructUnmarshalPartially`: -75% B/op (64 to 16), -1 alloc/op, -6.68% time.
- `Marshal` starts its encode buffer at 64 bytes, skipping early backing-array doubling on small payloads. The returned slice still owns its array. Aliasing matches upstream.
- `AppendMarshal` and `Encoder.Append` write into caller-owned buffers. Warm buffers show zero allocs per op on scalar and struct benchmarks.
- `byteWriter.WriteByte` writes through a 1-byte field instead of allocating a one-element slice per call. `BenchmarkDiscard`: -100% B/op and allocs/op.

`GetEncoder`, `PutEncoder`, `GetDecoder`, and `PutDecoder` behave as before. Reuse benchmarks are in `pkg/msgpack/bench_test.go`.

## Install

Go 1.27.1 or newer.

```bash
go get github.com/Quad4-Software/msgpack/v5@latest
```

For local development against a checkout, point a replace directive at it:

```go
replace github.com/Quad4-Software/msgpack/v5 => ../msgpack
```

Import:

```go
import "github.com/Quad4-Software/msgpack/v5/pkg/msgpack"
```

Source is under `pkg/msgpack`. msgpcode is at `pkg/msgpack/msgpcode`.

## Features

- Primitives, arrays, maps, structs, `time.Time`, `interface{}`
- `Marshal` for one-shot encoding, `AppendMarshal` and `Encoder.Append` for reusable output buffers
- App Engine `datastore.Key` and `Cursor` through `extra/msgpappengine`, an optional module
- `CustomEncoder` and `CustomDecoder`
- Extensions, msgpack struct tags, omitempty, sorted map keys, array-encoded structs, `Decoder.Query`

## Layout

| Path | Contents |
|------|----------|
| `pkg/msgpack` | Public API |
| `pkg/msgpack/msgpcode` | Wire opcode constants |
| `extra/msgpappengine` | App Engine helpers, own go.mod |
| `go.work` | Root module plus msgpappengine for local tests |
| `scripts/ci` | Local CI scripts |

## Testing

Unit tests cover the encoder and decoder surface, structs, time, extensions, interning, and queries.

Property tests using pbt live in `pbt_test.go`. Fuzz targets and allocation-limit corpora are in `fuzz_test.go` and `pkg/msgpack/testdata/fuzz`. `stress_test.go` covers concurrent `Marshal` and `Unmarshal` under `-race`, large payloads, deep nesting, and pool reuse. `invariant_test.go` and `leak_test.go` cover nil and empty input, bit-exact extremes, pool aliasing, and preallocator goroutine retention.

```bash
go test -race ./...
make   # go vet plus tests
```

## License

BSD 2-clause. See [LICENSE](LICENSE). Original copyright remains with the vmihailenco authors. Fork maintenance is attributed here.
