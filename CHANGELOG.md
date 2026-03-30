## [5.5.0](https://git.quad4.io/Go-Libs/msgpack) (2026-03-30)

Quad4 fork: maintenance release (module path and repository layout). Upstream lineage remains [github.com/vmihailenco/msgpack](https://github.com/vmihailenco/msgpack) v5.4.1.

### Module and imports

- Module path is now `git.quad4.io/Go-Libs/msgpack/v5` (major `/v5` unchanged for Go compatibility).
- Import the API as `git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack`; wire codes as `.../pkg/msgpack/msgpcode`.
- `extra/msgpappengine` module: `git.quad4.io/Go-Libs/msgpack/extra/msgpappengine`, with `replace` to the root module for local builds.

### Toolchain and dependencies

- Go **1.25.8**; `go.work` includes the root module and `extra/msgpappengine`.
- `git.quad4.io/Go-Libs/pbt` for property-based tests (test-only).
- `github.com/stretchr/testify` **v1.11.1** (test-only: `require` / `suite`); `gopkg.in/yaml.v3` **v3.0.1** (transitive); `google.golang.org/appengine` **v1.6.8** in the App Engine extra module.

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
