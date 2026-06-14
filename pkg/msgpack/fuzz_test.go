package msgpack_test

import (
	"bytes"
	"reflect"
	"testing"

	"quad4/msgpack/v5/pkg/msgpack"
)

// FuzzMarshalUnmarshalRoundtrip exercises the encoder and decoder for a
// representative set of typed values. For every input it asserts that
// Marshal followed by Unmarshal yields a value equal to the original.
//
// The test deliberately mixes scalar, container, and key-shaped values so
// the fuzzer can stress field-tag parsing, length prefixes, and the
// interface{} fast paths in a single run.
func FuzzMarshalUnmarshalRoundtrip(f *testing.F) {
	f.Add("", []byte{}, int64(0), 0.0, "", "")
	f.Add("hello", []byte("world"), int64(-1), 1.5, "k", "v")
	f.Add("\x00\xff", []byte{0xff, 0xfe, 0xfd}, int64(1<<31), -3.14, "", "x")

	f.Fuzz(func(t *testing.T, s string, b []byte, i int64, fl float64, k, v string) {
		check := func(name string, in, out any) {
			t.Helper()
			data, err := msgpack.Marshal(in)
			if err != nil {
				t.Fatalf("%s: marshal: %v", name, err)
			}
			if err := msgpack.Unmarshal(data, out); err != nil {
				t.Fatalf("%s: unmarshal: %v", name, err)
			}
		}

		var (
			outS  string
			outB  []byte
			outI  int64
			outF  float64
			outM  map[string]string
			outSl []string
		)

		check("string", s, &outS)
		if outS != s {
			t.Fatalf("string roundtrip: got %q want %q", outS, s)
		}

		check("bytes", b, &outB)
		if !bytes.Equal(outB, b) && !(len(outB) == 0 && len(b) == 0) {
			t.Fatalf("bytes roundtrip: got %x want %x", outB, b)
		}

		check("int64", i, &outI)
		if outI != i {
			t.Fatalf("int64 roundtrip: got %d want %d", outI, i)
		}

		check("float64", fl, &outF)
		if outF != fl {
			t.Fatalf("float64 roundtrip: got %v want %v", outF, fl)
		}

		m := map[string]string{k: v}
		check("map", m, &outM)
		if !reflect.DeepEqual(outM, m) {
			t.Fatalf("map roundtrip: got %#v want %#v", outM, m)
		}

		sl := []string{s, k, v}
		check("slice", sl, &outSl)
		if !reflect.DeepEqual(outSl, sl) {
			t.Fatalf("slice roundtrip: got %#v want %#v", outSl, sl)
		}
	})
}

// FuzzUnmarshalArbitrary feeds arbitrary bytes to Unmarshal against multiple
// destination types. The decoder must never panic; any error is acceptable.
//
// This guards the parser against malformed length prefixes, truncated
// payloads, malicious extension types, and oversized container headers.
func FuzzUnmarshalArbitrary(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xc0})
	f.Add([]byte{0xc1})
	f.Add([]byte{0x90})
	f.Add([]byte{0x80})
	f.Add([]byte{0xa3, 'a', 'b', 'c'})

	f.Fuzz(func(t *testing.T, data []byte) {
		var (
			any  any
			s    string
			b    []byte
			i    int64
			f64  float64
			m    map[string]interface{}
			sl   []interface{}
			strm map[string]string
		)
		_ = msgpack.Unmarshal(data, &any)
		_ = msgpack.Unmarshal(data, &s)
		_ = msgpack.Unmarshal(data, &b)
		_ = msgpack.Unmarshal(data, &i)
		_ = msgpack.Unmarshal(data, &f64)
		_ = msgpack.Unmarshal(data, &m)
		_ = msgpack.Unmarshal(data, &sl)
		_ = msgpack.Unmarshal(data, &strm)
	})
}

// FuzzDecoderQuery exercises the dotted-path Query API on arbitrary bytes.
// The decoder must never panic regardless of the input shape.
func FuzzDecoderQuery(f *testing.F) {
	f.Add([]byte{}, "")
	f.Add([]byte{0x90}, "0")
	f.Add([]byte{0x80}, "key")
	f.Add([]byte{0x82, 0xa1, 'a', 0x01, 0xa1, 'b', 0x02}, "a")

	f.Fuzz(func(t *testing.T, data []byte, query string) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		_, _ = dec.Query(query)
	})
}

// FuzzDecodeIntoStruct hammers the struct decoder with arbitrary input
// targeting a payload that mixes scalars, slices, maps, and time. The
// struct decoder uses the field cache (typeDecMap) and the preallocator;
// any panic here would indicate corruption of either.
type fuzzStructPayload struct {
	A int64             `msgpack:"a"`
	B string            `msgpack:"b"`
	C []byte            `msgpack:"c"`
	D []int             `msgpack:"d"`
	E map[string]string `msgpack:"e"`
	F float64           `msgpack:"f"`
}

func FuzzDecodeIntoStruct(f *testing.F) {
	src := fuzzStructPayload{A: 7, B: "hi", C: []byte{1, 2, 3}, D: []int{4, 5}, E: map[string]string{"k": "v"}, F: 1.5}
	seed, err := msgpack.Marshal(src)
	if err != nil {
		f.Fatalf("seed marshal: %v", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte{0x80})
	f.Add([]byte{0x81, 0xa1, 'a', 0xff})

	f.Fuzz(func(t *testing.T, data []byte) {
		var dst fuzzStructPayload
		_ = msgpack.Unmarshal(data, &dst)
	})
}

// FuzzDecodeExtHeader exercises the ext-header parser. Forged ext lengths
// must error rather than panic; the decoder must never read past the end
// of input or allocate a multi-gigabyte buffer for a forged Ext32 length.
func FuzzDecodeExtHeader(f *testing.F) {
	f.Add([]byte{0xd4, 0x01, 0x42})                   // FixExt1
	f.Add([]byte{0xd6, 0xff, 0x00, 0x00, 0x00, 0x00}) // FixExt4 timeExtID
	f.Add([]byte{0xc7, 0x00, 0x01})                   // Ext8 zero-length
	f.Add([]byte{0xc9, 0xff, 0xff, 0xff, 0xff, 0x01}) // Ext32 huge length

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		_, _, _ = dec.DecodeExtHeader()
	})
}

// FuzzDecodeTime targets the time-extension decoder which dispatches over
// FixedArray2, RFC3339 strings, and the ext encoding (4/8/12 bytes plus
// the NodeJS-style ext id 13). All paths must reject hostile input
// without panicking.
func FuzzDecodeTime(f *testing.F) {
	f.Add([]byte{0xd6, 0xff, 0x00, 0x00, 0x00, 0x00})                                                       // 4-byte ext, sec only
	f.Add([]byte{0xd7, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})                               // 8-byte ext
	f.Add([]byte{0xc7, 0x0c, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) // 12-byte ext
	f.Add([]byte{0x92, 0x00, 0x00})                                                                         // legacy fixarray-2
	f.Add(append([]byte{0xb4}, []byte("2026-04-18T00:00:00Z")...))                                          // RFC3339 string

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		_, _ = dec.DecodeTime()
	})
}

// FuzzDecodeInternedString exercises the interned-string path: ext-coded
// dictionary references and string-coded entries that grow the dict.
// The maxDictLen guard must hold; out-of-range index references must
// error instead of indexing past d.dict.
func FuzzDecodeInternedString(f *testing.F) {
	f.Add([]byte{0xa3, 'a', 'b', 'c'})
	f.Add([]byte{0xd4, 0x80, 0x00})                   // FixExt1, internedStringExtID, idx 0
	f.Add([]byte{0xd6, 0x80, 0xff, 0xff, 0xff, 0xff}) // FixExt4 with huge index

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		dec.UseInternedStrings(true)
		var s string
		_ = dec.Decode(&s)
	})
}

// FuzzDecodeSkip drives Skip over arbitrary input. Skip is the basis of
// the Query API and of struct field skipping, so a panic here amplifies
// across the rest of the surface.
func FuzzDecodeSkip(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xc0})
	f.Add([]byte{0x90})
	f.Add([]byte{0x80})
	f.Add([]byte{0xdc, 0x00, 0x10})

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		_ = dec.Skip()
	})
}

// FuzzDecodeMulti targets the variadic decode path. Most network framings
// (Redis cluster, NSQ, Tarantool) read sequences rather than single
// values; a panic here would propagate to every consumer.
func FuzzDecodeMulti(f *testing.F) {
	f.Add([]byte{0x01, 0x02})
	f.Add([]byte{0xc0, 0xc0, 0xc0})

	f.Fuzz(func(t *testing.T, data []byte) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		var a, b, c any
		_ = dec.DecodeMulti(&a, &b, &c)
	})
}

// FuzzDecodeLengthHeaders targets raw length-header APIs directly to stress
// overflow checks and malformed/truncated header handling.
func FuzzDecodeLengthHeaders(f *testing.F) {
	f.Add(byte(0), []byte{0xc6, 0xff, 0xff, 0xff, 0xff})       // bin32 max
	f.Add(byte(1), []byte{0xdd, 0xff, 0xff, 0xff, 0xff})       // array32 max
	f.Add(byte(2), []byte{0xdf, 0xff, 0xff, 0xff, 0xff})       // map32 max
	f.Add(byte(3), []byte{0xc9, 0xff, 0xff, 0xff, 0xff, 0x01}) // ext32 max + type
	f.Add(byte(4), []byte{0xc9, 0x00, 0x00, 0x00, 0x01})       // truncated ext

	f.Fuzz(func(t *testing.T, mode byte, data []byte) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		switch mode % 5 {
		case 0:
			_, _ = dec.DecodeBytesLen()
		case 1:
			_, _ = dec.DecodeArrayLen()
		case 2:
			_, _ = dec.DecodeMapLen()
		case 3:
			_, _, _ = dec.DecodeExtHeader()
		default:
			var s string
			_ = dec.Decode(&s)
		}
	})
}

// FuzzDecodeDepthGuard targets deeply nested arrays under varying depth
// limits; the decoder must return an error, not panic or overflow stack.
func FuzzDecodeDepthGuard(f *testing.F) {
	f.Add(byte(16), byte(32))
	f.Add(byte(64), byte(32))
	f.Add(byte(96), byte(64))
	f.Add(byte(128), byte(0))

	f.Fuzz(func(t *testing.T, depthByte, limitByte byte) {
		depth := int(depthByte) + 1
		limit := int(limitByte)

		data := make([]byte, 0, depth+1)
		for range depth {
			data = append(data, 0x91)
		}
		data = append(data, 0xc0)

		dec := msgpack.NewDecoder(bytes.NewReader(data))
		dec.SetDecodeDepthLimit(limit)
		var out any
		_ = dec.Decode(&out)

		dec = msgpack.NewDecoder(bytes.NewReader(data))
		dec.SetDecodeDepthLimit(limit)
		_ = dec.Skip()
	})
}
