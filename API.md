# msgpack API reference

Reference for `quad4/msgpack/v5/pkg/msgpack`. Designed to be useful both as human documentation and as a single grounding document for LLM coding agents that need to write or review code against this library.

The wire format and public API are 1:1 with upstream `github.com/vmihailenco/msgpack/v5@v5.4.1`. Migration is purely an import-path change; see [README.md](README.md#drop-in-migration-from-githubcomvmihailencomsgpackv5).

## Quick reference

```go
import "quad4/msgpack/v5/pkg/msgpack"
```

| Task | Call |
|------|------|
| Encode a value to bytes | `b, err := msgpack.Marshal(v)` |
| Encode into a reusable buffer | `b, err := msgpack.AppendMarshal(dst, v)` |
| Reuse encoder + buffer | `b, err := enc.Append(dst, v)` |
| Decode bytes into a value | `err := msgpack.Unmarshal(b, &v)` |
| Stream encode | `enc := msgpack.NewEncoder(w); enc.Encode(v)` |
| Stream decode | `dec := msgpack.NewDecoder(r); dec.Decode(&v)` |
| Reuse encoder/decoder | `enc := msgpack.GetEncoder(); ... msgpack.PutEncoder(enc)` |
| Pull a typed value | `n, err := dec.DecodeInt64()` (and friends) |
| Skip a value | `err := dec.Skip()` |
| Path query | `vs, err := dec.Query("a.*.b")` |
| Custom struct tag | `enc.SetCustomStructTag("json")` |
| Sort map keys | `enc.SetSortMapKeys(true)` |
| Compact ints/floats | `enc.UseCompactInts(true); enc.UseCompactFloats(true)` |
| As-array structs | `enc.UseArrayEncodedStructs(true)` |
| Strict struct decode | `dec.DisallowUnknownFields(true)` |
| Disable alloc limits | `dec.DisableAllocLimit(true)` (untrusted input: leave off) |
| Register extension type | `msgpack.RegisterExt(extID, (*T)(nil))` |
| Get on-wire codes | subpackage `msgpack/msgpcode` |

Top-level types: `Marshaler`, `Unmarshaler`, `CustomEncoder`, `CustomDecoder`, `Encoder`, `Decoder`, `RawMessage`.

## Top-level functions

### `Marshal(v any) ([]byte, error)`

Encodes `v` to a fresh byte slice using a pooled `*Encoder`. The returned slice owns its backing array and is safe to retain.

```go
type Item struct {
    Foo string
}
b, err := msgpack.Marshal(&Item{Foo: "bar"})
```

### `AppendMarshal(dst []byte, v any) ([]byte, error)`

Encodes `v` into `dst[:0]` and returns the resulting bytes. The result may reuse `dst`'s backing array, which lets callers keep a hot scratch buffer and remove the output-slice allocation in repeated marshal loops.

```go
dst := make([]byte, 0, 1024)
for _, item := range items {
    dst, err = msgpack.AppendMarshal(dst, item)
    if err != nil {
        return err
    }
    // write dst...
}
```

### `(*Encoder).Append(dst []byte, v any) ([]byte, error)`

Encodes `v` into `dst[:0]` using the receiver and returns the resulting bytes. Reusing both the encoder and the destination buffer is the lowest-allocation path for repeated small encodes and reaches zero allocs/op on hot paths with stable payload shapes.

```go
enc := msgpack.NewEncoder(nil)
dst := make([]byte, 0, 1024)
for _, n := range []int64{1, 2, 3} {
    dst, err = enc.Append(dst, n)
    if err != nil {
        return err
    }
    // write dst...
}
```

### `Unmarshal(data []byte, v any) error`

Decodes `data` into `v`. `v` must be a pointer. Uses pooled `*bytes.Reader` and `*Decoder` instances internally; the pool is reset to `nil` between calls and never aliases caller data.

```go
var item Item
err := msgpack.Unmarshal(b, &item)
```

`Unmarshal` enables value preallocation (`UsePreallocateValues(true)`) by default for the duration of the call; allocation limits are also enabled, see [Security flags](#security-flags).

### `Version() string`

Returns the current release version of this fork (for example `"5.6.0"`).

## `Encoder`

```go
enc := msgpack.NewEncoder(w io.Writer) *Encoder
```

Pooled variant for hot paths:

```go
enc := msgpack.GetEncoder()
enc.Reset(w)
defer msgpack.PutEncoder(enc)
```

If `w` does not implement `io.ByteWriter`, the encoder wraps it in a per-encoder `byteWriter` whose `WriteByte` writes through a 1-byte field on the wrapper struct (no per-call allocation).

### Encoding entry points

| Method | Description |
|--------|-------------|
| `Encode(v any) error` | Type-switches on common Go types, falls back to `EncodeValue` (reflection) for the rest. |
| `EncodeMulti(v ...any) error` | Encodes a sequence of values back-to-back. |
| `EncodeValue(reflect.Value) error` | Reflection-based entry point for already-typed values. |

### Scalar / well-known type writers

| Method | Wire type |
|--------|-----------|
| `EncodeNil()` | nil |
| `EncodeBool(bool)` | bool |
| `EncodeUint8(uint8)` ... `EncodeUint64(uint64)`, `EncodeUint(uint64)` | uint family |
| `EncodeInt8(int8)` ... `EncodeInt64(int64)`, `EncodeInt(int64)` | int family |
| `EncodeFloat32(float32)`, `EncodeFloat64(float64)` | float |
| `EncodeString(string)`, `EncodeBytes([]byte)` | str / bin |
| `EncodeBytesLen(int)`, `EncodeArrayLen(int)`, `EncodeMapLen(int)` | header-only writers; pair with element writes |
| `EncodeMap(map[string]any)`, `EncodeMapSorted(map[string]any)` | map |
| `EncodeTime(time.Time)`, `EncodeDuration(time.Duration)` | time ext / int64 |
| `EncodeExtHeader(extID int8, extLen int)` | extension header (write `extLen` body bytes after) |

### Encoder options

All option setters return after applying the flag; `SetSortMapKeys` returns `*Encoder` for chaining. Flags persist on the encoder until reset via `Reset` / `ResetDict` / `ResetWriter`.

| Option | Effect |
|--------|--------|
| `SetSortMapKeys(on bool) *Encoder` | Encode `map[string]string`, `map[string]bool`, `map[string]any` keys in lexicographic order (deterministic output). |
| `SetCustomStructTag(tag string)` | Use `tag` as the fallback struct tag if `msgpack:"..."` is absent (commonly `"json"`). |
| `SetOmitEmpty(on bool)` | Treat all string-keyed struct fields as `omitempty` by default. |
| `UseArrayEncodedStructs(on bool)` | Encode structs as msgpack arrays (positional) instead of maps (named). Decoder side reads either form. |
| `UseCompactInts(on bool)` | Pick the smallest int type that holds the value (e.g. `int8` for `42`). Saves bytes; semantics unchanged. |
| `UseCompactFloats(on bool)` | Encode floats whose value is an integer as the matching int type. |
| `UseInternedStrings(on bool)` | Reuse repeated strings via an in-stream dictionary; pair with `Decoder.UseInternedStrings(true)`. |

### Reset / dict

```go
enc.Reset(w)                       // discard buffered state, switch writer
enc.ResetDict(w, dict)             // also seed the intern dict
enc.ResetWriter(w)                 // switch writer only
enc.WithDict(dict, func(*Encoder) error) error  // run fn with a dict, restore previous after
```

### Example: streaming encode

```go
var buf bytes.Buffer
enc := msgpack.NewEncoder(&buf)
enc.SetSortMapKeys(true)
enc.UseCompactInts(true)
if err := enc.EncodeMulti("hello", 42, map[string]any{"k": "v"}); err != nil {
    return err
}
```

## `Decoder`

```go
dec := msgpack.NewDecoder(r io.Reader) *Decoder
```

If `r` already implements `io.ByteScanner`, it is used directly; otherwise it is wrapped in a `bufio.Reader`. Pooled variant:

```go
dec := msgpack.GetDecoder()
dec.Reset(r)
defer msgpack.PutDecoder(dec)
```

### Decoding entry points

| Method | Description |
|--------|-------------|
| `Decode(v any) error` | Decode into `v` (must be pointer). |
| `DecodeMulti(v ...any) error` | Decode a sequence of values back-to-back. |
| `DecodeValue(reflect.Value) error` | Reflection-based entry point. |
| `DecodeInterface() (any, error)` | Decode into the inferred Go type for the next value. |
| `DecodeInterfaceLoose() (any, error)` | Same but normalise int sizes to `int64` / `uint64` and float to `float64`. |

### Scalar readers

| Method | Notes |
|--------|-------|
| `DecodeNil() error`, `DecodeBool() (bool, error)` | |
| `DecodeUint8/16/32/64`, `DecodeUint`, `DecodeInt8/16/32/64`, `DecodeInt` | Integer family. Floats coded as `Float`/`Double` are accepted if finite, integer-valued, and in range. |
| `DecodeFloat32`, `DecodeFloat64` | float family |
| `DecodeString`, `DecodeBytes` | str / bin |
| `DecodeBytesLen`, `DecodeArrayLen`, `DecodeMapLen`, `DecodeExtHeader` | header-only readers |
| `DecodeMap`, `DecodeUntypedMap`, `DecodeTypedMap` | map variants |
| `DecodeSlice`, `DecodeTime`, `DecodeDuration` | well-known types |
| `DecodeRaw() (RawMessage, error)` | capture raw msgpack bytes for the next value |

### Stream control

| Method | Description |
|--------|-------------|
| `Skip() error` | Read and discard the next msgpack value (handles nested arrays/maps/ext). |
| `PeekCode() (byte, error)` | Inspect the next opcode without consuming it. |
| `ReadFull(buf []byte) error` | Read exactly `len(buf)` raw bytes (escape hatch for custom decoders). |
| `Buffered() io.Reader` | Underlying buffered reader (when wrapped in `bufio.Reader`). |

### Decoder options

| Option | Effect |
|--------|--------|
| `UseLooseInterfaceDecoding(on bool)` | When decoding into `any`, normalise ints to `int64`/`uint64` and floats to `float64`. |
| `SetCustomStructTag(tag string)` | Like the encoder's variant. |
| `DisallowUnknownFields(on bool)` | Error if a struct destination receives a key it does not recognise. |
| `UseInternedStrings(on bool)` | Recognise interned-string ext entries and grow `dec.dict`. |
| `UsePreallocateValues(on bool)` | Lazy `sync.Pool`-backed allocation for `reflect.New(t)`. Enabled automatically by `Unmarshal`. |
| `DisableAllocLimit(on bool)` | Disable per-decoder allocation caps (`bytesAllocLimit`, `sliceAllocLimit`, `maxMapSize`). Only enable for trusted input; see [Security flags](#security-flags). |
| `SetDecodeDepthLimit(limit int)` | Cap nested decode/skip recursion depth to defend against stack-exhaustion payloads. `limit <= 0` restores the default. |
| `SetMapDecoder(fn func(*Decoder) (any, error))` | Override the default `interface{}` map decoder. |

### Reset / dict

```go
dec.Reset(r)                       // discard buffered state, switch reader
dec.ResetDict(r, dict)             // also seed the intern dict
dec.ResetReader(r)                 // switch reader only
dec.WithDict(dict, func(*Decoder) error) error
```

## Struct tags

```go
type Item struct {
    ID        int64  `msgpack:"id"`
    Name      string `msgpack:"name,omitempty"`
    Internal  string `msgpack:"-"`            // skip
    Data      []byte `msgpack:"data,as_string"`
    Special   uint   `msgpack:"'has,comma'"`  // single-quoted name with comma
    _msgpack  struct{} `msgpack:",as_array"`  // whole struct as positional array
}
```

Recognised tag suffixes:

| Suffix | Behaviour |
|--------|-----------|
| `,omitempty` | Omit zero values. The struct-level marker (`_msgpack struct{} \`msgpack:",omitempty"\``) applies to all string-keyed fields. |
| `,as_array` | Encode the entire struct as an array (only valid on the `_msgpack struct{}` marker). |
| `,intern` | Intern this string field. |
| `,as_string` | Encode the `[]byte` field as a msgpack `str` instead of `bin`. |
| `,extension` | Use the ext header. |

Field name `-` skips the field. Single-quoted names allow commas / colons / dots in the on-wire key.

## Custom encoders

Three independent extension hooks are recognised, in priority order:

1. **`CustomEncoder` / `CustomDecoder`** — full streaming control over an `*Encoder` / `*Decoder`. Lowest overhead.
2. **`Marshaler` / `Unmarshaler`** — return / consume the encoded body bytes. Pair with `RegisterExt` for ext types.
3. **`encoding.BinaryMarshaler` / `encoding.BinaryUnmarshaler`** and **`encoding.TextMarshaler` / `encoding.TextUnmarshaler`** — interoperate with the standard library.

### `CustomEncoder` / `CustomDecoder`

```go
type customStruct struct {
    S string
    N int
}

func (s *customStruct) EncodeMsgpack(enc *msgpack.Encoder) error {
    return enc.EncodeMulti(s.S, s.N)
}

func (s *customStruct) DecodeMsgpack(dec *msgpack.Decoder) error {
    return dec.DecodeMulti(&s.S, &s.N)
}
```

### `Marshaler` / `Unmarshaler`

```go
type Object struct {
    Field int
}

func (o *Object) MarshalMsgpack() ([]byte, error)  { return msgpack.Marshal(o.Field) }
func (o *Object) UnmarshalMsgpack(b []byte) error  { return msgpack.Unmarshal(b, &o.Field) }
```

## Extension types

Extensions are user-defined ext IDs with a typed payload. Register once, decode via `interface{}` or directly into the registered type.

```go
type EventTime struct{ time.Time }

func (tm *EventTime) MarshalMsgpack() ([]byte, error) {
    b := make([]byte, 8)
    binary.BigEndian.PutUint32(b, uint32(tm.Unix()))
    binary.BigEndian.PutUint32(b[4:], uint32(tm.Nanosecond()))
    return b, nil
}

func (tm *EventTime) UnmarshalMsgpack(b []byte) error {
    if len(b) != 8 { return fmt.Errorf("len %d, want 8", len(b)) }
    tm.Time = time.Unix(int64(binary.BigEndian.Uint32(b)), int64(binary.BigEndian.Uint32(b[4:])))
    return nil
}

msgpack.RegisterExt(1, (*EventTime)(nil))

b, _ := msgpack.Marshal(&EventTime{time.Unix(123456789, 123)})

var v any
_ = msgpack.Unmarshal(b, &v)            // v is *EventTime
_, _ = v.(*EventTime), nil

var tm EventTime
_ = msgpack.Unmarshal(b, &tm)           // direct decode also works
```

For full control over the extension wire layout (skipping the `MarshalMsgpack`/`UnmarshalMsgpack` adapter):

```go
msgpack.RegisterExtEncoder(extID int8, value any,
    encoder func(enc *Encoder, v reflect.Value) ([]byte, error))

msgpack.RegisterExtDecoder(extID int8, value any,
    decoder func(dec *Decoder, v reflect.Value, extLen int) error)

msgpack.UnregisterExt(extID int8)
```

## Time

`time.Time` is encoded via the standard msgpack `-1` ext (4 / 8 / 12 byte forms depending on range). Decoding additionally accepts:

- The legacy 2-element fixarray (sec, nsec) form.
- RFC3339 strings.
- The NodeJS `13` ext id.

```go
b, _ := msgpack.Marshal(time.Now())
var t time.Time
_ = msgpack.Unmarshal(b, &t)

enc := msgpack.NewEncoder(w)
_ = enc.EncodeTime(t)

dec := msgpack.NewDecoder(r)
t2, _ := dec.DecodeTime()
```

`time.Duration` is encoded as `int64` nanoseconds.

## Query API

`Decoder.Query(query string) ([]any, error)` extracts values from a msgpack stream by dotted path. Path components are map keys, array indices, or `*` (all elements of an array).

```go
b, _ := msgpack.Marshal([]map[string]any{
    {"id": 1, "attrs": map[string]any{"phone": 12345}},
    {"id": 2, "attrs": map[string]any{"phone": 54321}},
})

dec := msgpack.NewDecoder(bytes.NewBuffer(b))
phones, _ := dec.Query("*.attrs.phone")  // -> [12345 54321]

dec.Reset(bytes.NewBuffer(b))
second, _ := dec.Query("1.attrs.phone")  // -> [54321]
```

Query consumes the entire input stream once; reset the decoder before issuing another query against the same data.

## `RawMessage`

`RawMessage` captures the raw msgpack bytes of a value for deferred decoding (analogous to `json.RawMessage`).

```go
type Envelope struct {
    Type    string
    Payload msgpack.RawMessage
}

var env Envelope
_ = msgpack.Unmarshal(data, &env)

switch env.Type {
case "user":
    var u User
    _ = msgpack.Unmarshal(env.Payload, &u)
}
```

## Pooled fast path

For high-throughput services, reuse encoders and decoders:

```go
func encodeFast(v any) ([]byte, error) {
    enc := msgpack.GetEncoder()
    var buf bytes.Buffer
    enc.Reset(&buf)
    err := enc.Encode(v)
    msgpack.PutEncoder(enc)
    return buf.Bytes(), err
}

func decodeFast(data []byte, v any) error {
    dec := msgpack.GetDecoder()
    dec.Reset(bytes.NewReader(data))
    err := dec.Decode(v)
    msgpack.PutDecoder(dec)
    return err
}
```

`Marshal` and `Unmarshal` already do this internally; the manual form is useful only when the caller wants to set non-default options on the pooled encoder/decoder.

## Security flags

The decoder enforces three caps when `DisableAllocLimit(false)` (the default):

| Constant | Meaning | Default |
|----------|---------|---------|
| `bytesAllocLimit` | Initial backing array for `bin` / `str` reads. | 1 MiB |
| `sliceAllocLimit` | Initial capacity for `array` reads (`[]any` and reflect-driven slices). | 1,000,000 elements |
| `maxMapSize` | Initial capacity hint for `map` reads. | 1,000,000 entries |

And one recursion guard:

| Setting | Meaning | Default |
|---------|---------|---------|
| `SetDecodeDepthLimit` | Maximum nested decode/skip depth before returning an error. | 10,000 |

Reads still grow incrementally as real bytes arrive, so a well-formed payload above these caps round-trips correctly when limits are disabled. With limits enabled, a hostile `bin32` / `str32` / `array32` / `map32` length prefix cannot trick the decoder into allocating multiple gigabytes up front before the underlying short read fails.

```go
dec := msgpack.NewDecoder(untrusted)        // limits ON (recommended)

dec := msgpack.NewDecoder(trusted)
dec.DisableAllocLimit(true)                 // limits OFF (only for trusted input)
```

`Unmarshal` calls leave limits enabled.

## Wire opcodes (`msgpcode` subpackage)

```go
import "quad4/msgpack/v5/pkg/msgpack/msgpcode"
```

Constants: `Nil`, `False`, `True`, `Bin8`/`Bin16`/`Bin32`, `Ext8`/`Ext16`/`Ext32`, `Float`/`Double`, `Uint8`/`Uint16`/`Uint32`/`Uint64`, `Int8`/`Int16`/`Int32`/`Int64`, `FixExt1`/`FixExt2`/`FixExt4`/`FixExt8`/`FixExt16`, `Str8`/`Str16`/`Str32`, `Array16`/`Array32`, `Map16`/`Map32`, plus the fixed prefixes (`PosFixedNumLow`, `FixedMapLow`, `FixedArrayLow`, `FixedStrLow`, `NegFixedNumLow`).

Predicates: `IsFixedNum`, `IsFixedMap`, `IsFixedArray`, `IsFixedString`, `IsString`, `IsBin`, `IsFixedExt`, `IsExt`.

## Errors and invariants

- `Unmarshal(nil, &v)` and `Unmarshal([]byte{}, &v)` return an error and never panic.
- `Marshal(nil)` is a single `msgpcode.Nil` byte.
- Pool reuse never leaks the previous caller's data into a subsequent `Unmarshal`.
- Decoding `NaN` or `±Inf` round-trips bit-exact; converting them into an integer returns an error rather than silently truncating.
- Float-coded values round-trip into integer destinations only when finite, integer-valued, and in range.
- The decoder never spawns background goroutines; the per-type preallocator is a `sync.Map` of `*sync.Pool` so idle entries are reclaimed by the GC.

## End-to-end example

```go
package main

import (
    "bytes"
    "fmt"

    "quad4/msgpack/v5/pkg/msgpack"
)

type User struct {
    ID    int64    `msgpack:"id"`
    Name  string   `msgpack:"name,omitempty"`
    Tags  []string `msgpack:"tags,omitempty"`
}

func main() {
    var buf bytes.Buffer
    enc := msgpack.NewEncoder(&buf)
    enc.SetSortMapKeys(true)
    enc.UseCompactInts(true)
    enc.SetOmitEmpty(true)

    if err := enc.Encode(&User{ID: 7, Name: "ada", Tags: []string{"admin"}}); err != nil {
        panic(err)
    }

    dec := msgpack.NewDecoder(&buf)
    dec.SetCustomStructTag("json")
    dec.DisallowUnknownFields(true)

    var u User
    if err := dec.Decode(&u); err != nil {
        panic(err)
    }
    fmt.Printf("%+v\n", u)
}
```

## Notes for LLM agents

- Always import the package as `msgpack`; the import path is `quad4/msgpack/v5/pkg/msgpack`. Do not generate `github.com/vmihailenco/...` paths in this codebase.
- Wire codes live in `quad4/msgpack/v5/pkg/msgpack/msgpcode`.
- Pass pointers to `Unmarshal` / `Decode` (`&v`, never `v`).
- Prefer `dec.Skip()` over rewriting parsing logic to discard a value; it correctly walks nested arrays, maps, and ext payloads.
- Do not call `DisableAllocLimit(true)` on input that came from the network or from disk without verifying the source.
- When emitting deterministic output (golden files, signatures, content-addressed storage), set `SetSortMapKeys(true)` and prefer `UseArrayEncodedStructs(true)` or the `,as_array` struct marker to avoid map-ordering noise.
- For struct types whose encoding must match an external `encoding/json` schema, set `SetCustomStructTag("json")` on both encoder and decoder.
- Custom types implementing `EncodeMsgpack` / `DecodeMsgpack` take precedence over `MarshalMsgpack` / `UnmarshalMsgpack`, which take precedence over `encoding.BinaryMarshaler` / `encoding.TextMarshaler`. Implement only one set per type.
- Floats that arrived from a JSON-style pipeline (`float64` carrying integer values) decode cleanly into integer fields; do not pre-convert in user code.
