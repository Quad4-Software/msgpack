## [5.8.0](https://github.com/Quad4-Software/msgpack) (2026-07-04)

### Security hardening

- Fixed a decode-recording buffer leak in `(*Decoder).DecodeRaw` and the `Unmarshaler` decode path (`pkg/msgpack/decode.go`, `pkg/msgpack/decode_value.go`):
  - When `Skip()` failed partway through recording, `d.rec` was left set on the `Decoder`.
  - Every subsequent read on that instance (including decoders drawn from the package pool) would then silently append into the abandoned buffer, growing without bound for the remaining lifetime of the decoder.
  - Both paths now clear `d.rec` on every exit via `defer`, including errors.

### Memory efficiency

- Capped scratch-buffer retention in the pooled encoder and decoder (`PutEncoder`, `PutDecoder` in `pkg/msgpack/encode.go` and `pkg/msgpack/decode.go`):
  - Buffers whose capacity exceeds `bytesAllocLimit` (1 MiB) are dropped or reset before the instance re-enters `sync.Pool`, so one legitimately large payload cannot pin multi-megabyte backing arrays for unrelated future callers.
  - Encoder `buf` is reset to the default 9-byte scratch rather than `nil` so `write1` through `write8` remain usable on the next `Get`.
- Removed the non-functional per-type preallocator in `pkg/msgpack/decode_typgen.go`:
  - The previous `sync.Map` of `*sync.Pool` never amortized allocation because decoded values were never returned to the pool; every `Get` was effectively `reflect.New(t)` plus map and pool overhead.
  - `(*Decoder).UsePreallocateValues` and its flag bit are retained for API compatibility but no longer change behavior.
  - `indirectNil` in `pkg/msgpack/types.go` now allocates with `reflect.New` directly.

### Performance

- Converted all wire-format opcodes in `pkg/msgpack/msgpcode` from package-level `var` to `const`:
  - Prevents runtime mutation of opcode values by importers.
  - Enables compile-time folding and better inlining on decode/encode hot paths.
  - `EncodeInt` retains a package-level `negFixedNumLow` helper because `int8(0xe0)` is not a valid constant conversion in Go.

### Fuzzing and tests

- Added `pkg/msgpack/internal_test.go` (white-box regressions):
  - `TestDecodeRawClearsRecOnError`, `TestDecodeRawClearsRecOnSuccess`, `TestDecodeRawErrorDoesNotPoisonSubsequentReads`
  - `TestUnmarshalValueClearsRecOnError`
  - `TestPutDecoderDropsOversizedBuffers`, `TestPutDecoderKeepsSmallBuffers`, `TestPooledDecoderDoesNotRetainHugePayloadCapacity`
  - `TestPutEncoderDropsOversizedBuffers`, `TestPutEncoderKeepsSmallBuffers`, `TestPooledEncoderDoesNotRetainHugeAppendCapacity`
  - `TestNewValueAllocatesDirectly`
- Extended `pkg/msgpack/fuzz_test.go`:
  - `FuzzDecodeRaw` (failed raw decode, then reuse the same decoder)
  - `FuzzUnmarshalArbitrary` now decodes into `RawMessage` as well
- Validation:
  - Full suite passes: `go test ./...`, `go test -race ./...`
  - New fuzz targets executed successfully (20s smoke runs on `FuzzDecodeRaw` and extended `FuzzUnmarshalArbitrary`).

## [5.7.0](https://github.com/Quad4-Software/msgpack) (2026-05-01)

### Performance

- Added reusable-output encode APIs:
  - `msgpack.AppendMarshal(dst, v)`
  - `(*msgpack.Encoder).Append(dst, v)`
- `Marshal` now routes through `AppendMarshal` while preserving default encoder semantics.
- Hot-path allocation removal:
  - Reused encoder-owned append writer state instead of creating per-call wrappers.
  - Removed reflection boxing in string-slice encode path.
  - Added addressable `time.Time` fast path in extension encoding.
- New benchmarks in `pkg/msgpack/bench_test.go`:
  - `BenchmarkStructAppendMarshalReuse`
  - `BenchmarkStructEncoderAppendReuse`
  - `BenchmarkIntEncoderAppendReuse`
- Representative measured results (amd64):
  - `StructAppendMarshalReuse`: ~314-318 ns/op, 0 B/op, 0 allocs/op
  - `StructEncoderAppendReuse`: ~297-305 ns/op, 0 B/op, 0 allocs/op
  - `IntEncoderAppendReuse`: ~13 ns/op, 0 B/op, 0 allocs/op

### Security hardening

- Added decode recursion-depth guard in `pkg/msgpack/decode.go`:
  - Default limit: `10000`
  - New API: `(*Decoder).SetDecodeDepthLimit(limit int)` (`limit <= 0` restores default)
  - Guard enforced across recursive decode and skip paths (`DecodeValue`, interface decode dispatch, `Skip`) to mitigate stack-exhaustion payloads.
- Added explicit `uint32 -> int` overflow protection for length/index parsing on 32-bit targets:
  - `str32` / `bin32`, `array32`, `map32`, `ext32`, and interned-string len/index paths now reject values that overflow `int` instead of wrapping.

### Fuzzing and tests

- Added `pkg/msgpack/security_test.go`:
  - `TestLengthPrefixOverflowGuards`
  - `TestDecodeDepthLimitGuards`
- Added fuzz targets in `pkg/msgpack/fuzz_test.go`:
  - `FuzzDecodeLengthHeaders`
  - `FuzzDecodeDepthGuard`
- Tightened allocation invariants in `pkg/msgpack/invariant_test.go`:
  - `TestInvariantAppendMarshalRoundTrip`
  - `TestInvariantAppendMarshalZeroAllocsWithWarmBuffer`
  - `TestInvariantEncoderAppendZeroAllocsWithWarmBuffer`
- Validation:
  - Full suite passes on default arch: `go test ./...`
  - 32-bit overflow/depth guards validated with `GOARCH=386 go test ...`
  - New fuzz targets executed successfully.

### API and docs

- `API.md` updated with `AppendMarshal`, `(*Encoder).Append`, and `SetDecodeDepthLimit`.
- `README.md` updated with zero-allocation append-path notes and new security-hardening details.

### Modernization

- Ran `go fix ./...` and accepted safe modernizations across touched files (for example `interface{}` -> `any`, range-loop simplifications, and canonical `//go:build` tags in `safe.go` / `unsafe.go`).

## [5.6.1](https://github.com/Quad4-Software/msgpack) (2026-04-21)

### Dependencies

- Removed **`github.com/stretchr/testify`** from the root module; tests now use the standard **`testing`** package only. Transitive test dependencies dropped from the main `go.sum` include **`github.com/davecgh/go-spew`**, **`github.com/pmezard/go-difflib`**, and **`gopkg.in/yaml.v3`**.

### Tests

- Added **`pkg/msgpack/testing_helpers_test.go`**: small **`t.Helper()`** helpers (`mustOK`, `mustErr`, `mustErrorString`, **`mustEqual`** for comparable types, **`mustDeepEqual`**, **`mustBytesEqual`**, **`mustTrue`**) so failures report the calling test line and repetitive error checks stay readable.
- **`pkg/msgpack/msgpack_test.go`**: encoder/decoder harness gains **`mustEncode`** / **`mustDecode`**; **`t.Run`** subtests for time round-trip, string maps, custom coders (**`TestCustomCoderRoundTrip`**), and omit-empty behaviour; table-driven **`TestMapStringString`** (formerly a single loop); clearer assertions for embedding, sorted-map stability, and unsupported map keys.
- **`pkg/msgpack/intern_test.go`**, **`pkg/msgpack/ext_test.go`**: switched to the shared helpers; **`TestResetDict`** split into named subtests.
- **`pkg/msgpack/types_test.go`**: **`TestEncoder`** and **`TestStringsBin`** use **`t.Run`** per case and the shared helpers where appropriate.

## [5.6.0](https://github.com/Quad4-Software/msgpack) (2026-04-19)

### Dependencies

- `quad4/tagparser/v2` **v2.1.0** ([Quad4 fork](https://quad4/tagparser)) replaces `github.com/vmihailenco/tagparser/v2` for struct tag parsing in `pkg/msgpack/types.go`; import path `.../pkg/tagparser`. The same change is reflected in the `extra/msgpappengine` submodule.

### Security and correctness

#### OOM via forged length prefix

The fuzz suite (`pkg/msgpack/fuzz_test.go`) found four hostile-input paths where a forged length prefix would request a multi-gigabyte allocation up front and OOM the process before the underlying short read could fail. All four now respect the existing `disableAllocLimitFlag` toggle and clamp the initial allocation to the documented per-decoder limit (`bytesAllocLimit`, `sliceAllocLimit`, `maxMapSize`); decoding still grows incrementally as real bytes arrive, so well-formed inputs above the limit continue to round-trip when limits are explicitly disabled via `Decoder.UseAllocLimitDisable(true)`.

- `decode_string.go`: `(*Decoder).bytesPtr` and `(*Decoder).bytes` now route through `readNGrow` (cap-bounded incremental reader) unless limits are disabled. Previously a `bin32` / `str32` length was allocated in one shot.
- `decode_slice.go`: `(*Decoder).decodeSlice` (the `interface{}` array path) clamps the initial backing-array capacity to `sliceAllocLimit`. Previously an `array32` length was passed straight to `make`.
- `decode_slice.go`: `decodeSliceValue` previously compared the disable flag against the literal `1`, but `disableAllocLimitFlag` is `1 << 1 == 2`. The comparison is now `!= 0`, restoring the documented behaviour and routing growth through `growSliceValue` in clamped chunks.
- `decode_map.go`: `(*Decoder).DecodeMap` and `(*Decoder).DecodeUntypedMap` now clamp the initial map size hint to `maxMapSize` unless limits are disabled. Previously a `map32` length sized the map directly.

#### Goroutine and memory leak in the per-type preallocator

`pkg/msgpack/decode_typgen.go` previously spawned one perpetual goroutine for every distinct `reflect.Type` ever decoded into. Each goroutine ran an unbounded loop pushing freshly-allocated `reflect.Value` instances into a buffered channel of length 256 and was never reaped, so a long-running process that ever decoded into N distinct Go types retained N goroutines (~8 KiB stack each) plus 256 N preallocated values for the lifetime of the process. The map of channels was also unbounded, and lookup held a global `RWMutex` on the hot decode path.

The preallocator is now backed by a `sync.Map` of `*sync.Pool`, one per type; the pool's `New` function lazily produces `reflect.New(t)`. This preserves the amortized allocation pattern (per-P caching), removes the goroutines entirely, removes the global mutex from the lookup path, and lets the GC drain idle entries during quiescent periods. Coverage:

- `pkg/msgpack/leak_test.go`: `TestNoGoroutineLeakOnDistinctTypes` (256 distinct generated struct types must add at most a few goroutines), `TestNoGoroutineLeakOnRepeatedDecode` (5000 decodes of one type), `TestConcurrentDistinctTypesPreallocate` (32 goroutines x 200 iterations across four types under `-race`), and `TestPoolDoesNotRetainCallerData` (the pooled `*bytes.Reader` must not leak the previous caller's slice into the next decode).

#### Float-coded values into integer destinations

`(*Decoder).int` and `(*Decoder).uint` in `decode_number.go` now accept `msgpcode.Float` and `msgpcode.Double` payloads when decoding into a Go integer destination. This unblocks the common JSON -> `map[string]any` -> msgpack -> struct round-trip pattern that previously errored with `invalid code=cb decoding int64` on every numeric field, because `encoding/json` decodes JSON numbers into `float64` regardless of the destination type.

Conversion is strict: a float is only accepted when it is finite, has no fractional component (`f == math.Trunc(f)`), and fits in the target's range. NaN, infinities, fractional values, negative values into `uint64`, and out-of-range magnitudes all return an error rather than silently truncating. Regression coverage lives in `pkg/msgpack/decode_number_more_test.go`.

These are correctness fixes, not API changes: the wire format and behaviour for previously accepted inputs are unchanged.

### Performance

Verified with `benchstat` over 5 runs at `-benchtime=2s`. Geomean wall time across the standard benchmark set is **-4.96%** with no allocation regressions:

- `BenchmarkDiscard`: **-100% B/op**, **-100% allocs/op** (2 allocs and 2 B per pair of writes -> 0). `byteWriter.WriteByte` now writes through a 1-byte field on the wrapper struct instead of allocating a fresh `[]byte{c}` on every call. The wrapper is only used when the underlying writer does not implement `io.ByteWriter`; for `bytes.Buffer`, `bufio.Writer`, and `io.Discard` it is bypassed entirely.
- `BenchmarkStructUnmarshal`: **-50% B/op** (96 -> 48), **-12.5% allocs/op** (8 -> 7), **-5.16% time**. `Unmarshal` now pools the `*bytes.Reader` used to wrap the input slice; the reader is reset to `nil` before being returned to the pool so it does not retain a reference to the caller's data.
- `BenchmarkStructUnmarshalPartially`: **-75% B/op** (64 -> 16), **-50% allocs/op** (2 -> 1), **-6.68% time**.
- `BenchmarkStructVmihailencoMsgpack` (full marshal + unmarshal round-trip): **-6.67% allocs/op** (15 -> 14), **-3.01% B/op**.
- `BenchmarkStructManual` (`CustomEncoder` / `CustomDecoder`): **-5.88% allocs/op** (17 -> 16), **-3.16% B/op**.
- `Marshal`: pre-grow the encode buffer to 64 bytes so the first one or two backing-array doublings are skipped for typical small payloads. The returned slice still owns its backing array (the buffer is not pooled), preserving existing aliasing semantics.

Marginal time deltas elsewhere (`MapStringInterfaceMsgpack` +3.5%, `StructVmihailencoMsgpack` +2.6%, `StringSlicePtr` +1.4%) are within run-to-run noise at this sample size and trade off for clear allocation savings.

### Tests

- `pkg/msgpack/fuzz_test.go`: `FuzzMarshalUnmarshalRoundtrip` (string, bytes, int, float, map, slice), `FuzzUnmarshalArbitrary` (must never panic on hostile input into typed and `interface{}` destinations), `FuzzDecoderQuery` (Query path safety on arbitrary bytes), `FuzzDecodeIntoStruct` (mixed-shape struct destination), `FuzzDecodeExtHeader` (`Ext8`/`Ext16`/`Ext32` and `FixExt*` length parsing), `FuzzDecodeTime` (RFC3339, legacy `FixedArray2`, and 4/8/12-byte ext encodings, including NodeJS ext id 13), `FuzzDecodeInternedString` (interned-string ext path and `dict` index bounds), `FuzzDecodeSkip` (the basis of struct field skipping and Query), and `FuzzDecodeMulti` (variadic decode path used by streaming framings). Regression corpora for the four allocation-limit fixes are committed under `pkg/msgpack/testdata/fuzz/FuzzUnmarshalArbitrary/` and re-checked on every `go test` run.
- `pkg/msgpack/stress_test.go`: concurrent `Marshal` / `Unmarshal` (`-race` clean across 32 goroutines x 200 iterations), large byte slice / string round-trips (2 MiB, gated on `-short`), 16-level nested map and slice round-trips, repeated reuse of pooled encoder/decoder with mutated flags.
- `pkg/msgpack/invariant_test.go`: `Unmarshal` of `nil` / empty input returns an error and never panics; `Marshal(nil)` is a single `msgpcode.Nil` byte; bit-exact round-trip of `int64` min, `uint64` max, `NaN`, and `-Inf`; pooled `*bytes.Reader` does not leak the previous caller's data into a subsequent `Unmarshal`.
- `pkg/msgpack/leak_test.go`: goroutine and pool aliasing regressions for the per-type preallocator (see "Goroutine and memory leak in the per-type preallocator" above).
- `pkg/msgpack/pbt_test.go`: extended with property tests for `[]int`, `[]string`, and `map[string]string` round-trips.
- `pkg/msgpack/bench_test.go`: parallel `BenchmarkMarshalParallel` / `BenchmarkUnmarshalParallel` exercising the encoder/decoder pools, `BenchmarkLargeByteSlice` (64 KiB), `BenchmarkLargeString` (64 KiB), `BenchmarkNestedMap` (5-level deep `map[string]any`), and `BenchmarkStructMarshalReuse` exercising `GetEncoder` / `PutEncoder`.

## [5.5.0](https://github.com/Quad4-Software/msgpack) (2026-04-18)

Quad4 fork: maintenance release (module path and repository layout). Upstream lineage remains [github.com/vmihailenco/msgpack](https://github.com/vmihailenco/msgpack) v5.4.1.

### Module and imports

- Module path is now `quad4/msgpack/v5` (major `/v5` unchanged for Go compatibility).
- Import the API as `quad4/msgpack/v5/pkg/msgpack`; wire codes as `.../pkg/msgpack/msgpcode`.
- `extra/msgpappengine` module: `github.com/Quad4-Software/msgpack/extra/msgpappengine`, with `replace` to the root module for local builds.

### Toolchain and dependencies

- Go **1.26.2**; `go.work` includes the root module and `extra/msgpappengine`.
- `quad4/pbt` for property-based tests (test-only).
- `github.com/vmihailenco/tagparser/v2` **v2.0.0** for struct tag parsing (`pkg/msgpack/types.go`).
- `google.golang.org/appengine` **v1.6.8** in the App Engine extra module; `github.com/golang/protobuf` **v1.5.4** and `google.golang.org/protobuf` **v1.36.11** (transitive via App Engine).

### Layout and repository

- Library sources moved from repository root to **`pkg/msgpack/`**; **`pkg/msgpack/msgpcode/`** holds opcode definitions.
- Added **`.gitea/workflows`** (`ci.yml`, `scan.yml`) and **`scripts/ci/`** (setup-go, gosec, govulncheck, trivy, test-all, scan-all, checkout) aligned with other Quad4 Go libraries.
- Removed legacy **`.github/`**, **`.travis.yml`**, **commitlint** / **npm** config files.
- **`Makefile`** runs `go vet ./...` and tests against `./...`.
- **`.gitignore`**: local `GOMODCACHE` / `GOCACHE` / `GOTMPDIR` cache dirs when building on constrained disks.

### Documentation and legal

- **`README.md`**: fork notice, Quad4 maintenance, install/import paths, layout table.
- **`LICENSE`**: retained upstream copyright; added Quad4 fork line.
- **`pkg/msgpack/doc.go`**: package overview for pkg.go.dev.

### Code

- Ran **`go fix ./...`** to apply available automated modernizations.

### Property-based tests

- **`pkg/msgpack/pbt_test.go`**: roundtrip properties for `[]byte`, `string`, and `map[string]int` via **pbt**.

## [5.4.1](https://github.com/vmihailenco/msgpack/compare/v5.4.0...v5.4.1) (2023-10-26)


### Bug Fixes

* **reflect:** not assignable to type ([edeaedd](https://github.com/vmihailenco/msgpack/commit/edeaeddb2d51868df8c6ff2d8a218b527aeaf5fd))



# [5.4.0](https://github.com/vmihailenco/msgpack/compare/v5.3.6...v5.4.0) (2023-10-01)



## [5.3.6](https://github.com/vmihailenco/msgpack/compare/v5.3.5...v5.3.6) (2023-10-01)


### Features

* allow overwriting time.Time parsing from extID 13 (for NodeJS Date) ([9a6b73b](https://github.com/vmihailenco/msgpack/commit/9a6b73b3588fd962d568715f4375e24b089f7066))
* apply omitEmptyFlag to empty structs ([e5f8d03](https://github.com/vmihailenco/msgpack/commit/e5f8d03c0a1dd9cc571d648cd610305139078de5))
* support sorted keys for map[string]bool ([690c1fa](https://github.com/vmihailenco/msgpack/commit/690c1fab9814fab4842295ea986111f49850d9a4))



## [5.3.5](https://github.com/vmihailenco/msgpack/compare/v5.3.4...v5.3.5) (2021-10-22)

- Allow decoding `nil` code as boolean false.

## v5

### Added

- `DecodeMap` is split into `DecodeMap`, `DecodeTypedMap`, and `DecodeUntypedMap`.
- New msgpack extensions API.

### Changed

- `Reset*` functions also reset flags.
- `SetMapDecodeFunc` is renamed to `SetMapDecoder`.
- `StructAsArray` is renamed to `UseArrayEncodedStructs`.
- `SortMapKeys` is renamed to `SetSortMapKeys`.

### Removed

- `UseJSONTag` is removed. Use `SetCustomStructTag("json")` instead.

## v4

- Encode, Decode, Marshal, and Unmarshal are changed to accept single argument. EncodeMulti and
  DecodeMulti are added as replacement.
- Added EncodeInt8/16/32/64 and EncodeUint8/16/32/64.
- Encoder changed to preserve type of numbers instead of chosing most compact encoding. The old
  behavior can be achieved with Encoder.UseCompactEncoding.

## v3.3

- `msgpack:",inline"` tag is restored to force inlining structs.

## v3.2

- Decoding extension types returns pointer to the value instead of the value. Fixes #153

## v3

- gopkg.in is not supported any more. Update import path to github.com/vmihailenco/msgpack.
- Msgpack maps are decoded into map[string]interface{} by default.
- EncodeSliceLen is removed in favor of EncodeArrayLen. DecodeSliceLen is removed in favor of
  DecodeArrayLen.
- Embedded structs are automatically inlined where possible.
- Time is encoded using extension as described in https://github.com/msgpack/msgpack/pull/209. Old
  format is supported as well.
- EncodeInt8/16/32/64 is replaced with EncodeInt. EncodeUint8/16/32/64 is replaced with EncodeUint.
  There should be no performance differences.
- DecodeInterface can now return int8/16/32 and uint8/16/32.
- PeekCode returns codes.Code instead of byte.
