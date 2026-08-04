# msgpack

Fork of [vmihailenco/msgpack](https://github.com/vmihailenco/msgpack) (v5 API), published as quad4/msgpack/v5.

Wire format and public API match upstream v5.4.1. Marshal, Unmarshal, Encoder, Decoder, struct tags, and options keep the same signatures. This fork adds security, correctness, and performance fixes without breaking changes.

## Migration from github.com/vmihailenco/msgpack/v5

Change the import:

```go
// before
import "github.com/vmihailenco/msgpack/v5"

// after
import "quad4/msgpack/v5/pkg/msgpack"
```

The package name is still msgpack, so call sites stay the same. You also need a replace directive (see Install). Plain `go get quad4/msgpack/v5@...` will not resolve.

App Engine helpers move to a separate module:

```go
// before
import "github.com/vmihailenco/msgpack/v5/msgpappengine"

// after
import "quad4/msgpack/v5/extra/msgpappengine"
```

Wire opcode constants (msgpcode) move to quad4/msgpack/v5/pkg/msgpack/msgpcode. Values are unchanged.

## Fixes vs upstream v5.4.1

Upstream v5.4.1 is the last tagged release on the original module. API stays the same. Differences:

### Security

Several decode paths trusted the on-wire length and allocated up front (bytes, bytesPtr, decodeSlice, decodeSliceValue, DecodeMap, DecodeUntypedMap). A hostile bin32, str32, array32, or map32 header could force a multi-gigabyte allocation before the short read failed. Those paths now clamp the first allocation to the documented per-decoder limit and grow as bytes arrive. Decoder.DisableAllocLimit(true) restores the old unbounded behaviour.

Decoder recursion has a depth cap (SetDecodeDepthLimit, default 10,000). Deeply nested payloads fail with an error instead of burning stack. Length parsing for str32, bin32, array32, map32, and ext32 rejects uint32 values that overflow int on 32-bit builds.

disableAllocLimitFlag was compared against the literal 1 in decodeSliceValue, but the flag is 1<<1 (2), so the typed-slice path never applied the limit. The check is now != 0.

The upstream cachedValue preallocator spawned one perpetual goroutine and a 256-slot buffered channel per distinct reflect.Type, and held a global sync.RWMutex on the hot decode path. Decoding into N Go types left N goroutines forever. This fork uses a sync.Map of sync.Pool values. Idle entries can be GC'd, the global mutex is off the lookup path, and goroutine count stays flat. See TestNoGoroutineLeakOnDistinctTypes and TestConcurrentDistinctTypesPreallocate in pkg/msgpack/leak_test.go.

### Correctness

Decoder int/uint paths accept Float and Double wire codes when the destination is a Go integer, if the value is finite, integer-valued, and in range. That covers the common JSON -> map[string]any -> msgpack -> struct round-trip (encoding/json stores numbers as float64). NaN, infinities, fractions, negatives into uint64, and out-of-range magnitudes still error.

Marshal and Unmarshal reuse buffers via sync.Pool. Wrappers are reset to nil before return so a pooled bytes.Reader cannot leak the previous caller's slice. See TestPoolDoesNotRetainCallerData and TestInvariantPoolDoesNotAlias.

### Performance

Geomean over five 2s benchstat runs against upstream v5.4.1:

- Unmarshal: pooled bytes.Reader. BenchmarkStructUnmarshal -50% B/op (96 -> 48), -1 alloc/op, -5.16% time. BenchmarkStructUnmarshalPartially -75% B/op (64 -> 16), -1 alloc/op, -6.68% time.
- Marshal: encode buffer starts at 64 bytes, skipping early backing-array doublings for small payloads. Returned slice still owns its array. Aliasing matches upstream.
- AppendMarshal and Encoder.Append: caller-owned destination buffers. Warm buffer: zero allocs/op on scalar and struct benches.
- byteWriter.WriteByte: writes through a 1-byte field instead of allocating []byte{c} each time. BenchmarkDiscard -100% B/op and allocs/op.
- GetEncoder / PutEncoder and GetDecoder / PutDecoder behave as before. Reuse benches are in pkg/msgpack/bench_test.go.

## Install

Go 1.26.5 or newer.

Module path is quad4/msgpack/v5. That is not a public proxy name and not a github.com import, so the toolchain needs a replace pointing at this repo or a local checkout.

In your go.mod:

```go
require quad4/msgpack/v5 v5.8.2

replace quad4/msgpack/v5 => github.com/Quad4-Software/msgpack v5.8.2
```

Local clone next to your project:

```go
require quad4/msgpack/v5 v5.8.2

replace quad4/msgpack/v5 => ../msgpack
```

Then:

```bash
go mod tidy
go mod vendor   # optional
go build -mod=vendor ./...
```

Import:

```go
import "quad4/msgpack/v5/pkg/msgpack"
```

Source is under pkg/msgpack. msgpcode is at pkg/msgpack/msgpcode.

## Features

- Primitives, arrays, maps, structs, time.Time, interface{}
- Marshal for convenience, AppendMarshal / Encoder.Append for reusable output buffers
- App Engine datastore.Key and Cursor via extra/msgpappengine (optional module)
- CustomEncoder / CustomDecoder
- Extensions, msgpack struct tags, omitempty, sorted map keys, array-encoded structs, Decoder.Query

## Layout

| Path | Purpose |
|------|---------|
| pkg/msgpack | Public API |
| pkg/msgpack/msgpcode | Wire opcode constants |
| extra/msgpappengine | App Engine helpers (own go.mod) |
| go.work | Root module + msgpappengine for local tests |
| scripts/ci | Local CI scripts |
| .github/workflows | CI and security workflows |

## Testing

Unit tests cover the encoder/decoder surface, structs, time, extensions, intern, and queries.

Property tests (pbt) live in pbt_test.go. Fuzz targets and allocation-limit corpora are in fuzz_test.go and pkg/msgpack/testdata/fuzz. stress_test.go covers concurrent Marshal/Unmarshal under -race, large payloads, deep nesting, and pool reuse. invariant_test.go and leak_test.go cover nil/empty input, bit-exact extremes, pool aliasing, and preallocator goroutine retention.

```bash
go test -race ./...
make   # go vet + tests
```

## License

BSD 2-clause. See [LICENSE](LICENSE). Original copyright remains with the vmihailenco authors. Fork maintenance is attributed here.
