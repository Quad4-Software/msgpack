package msgpack_test

// Golden wire-format vectors. Every Marshal output is compared byte for
// byte against the canonical encoding defined by the MessagePack spec,
// and each payload is decoded back into a value of the same Go type.
// These tests are the contract that the hardened decoder and encoder
// stay wire-compatible with upstream.

import (
	"bytes"
	"encoding/hex"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Quad4-Software/msgpack/v5/pkg/msgpack"
)

// wireGolden cases use Marshal. Fixed-width Go types (int8..64,
// uint8..64, float32/64) encode with their type-preserving codes.
// Plain int and uint encode via the canonical smallest-width path
// (EncodeInt/EncodeUint). int cases stay within int32 range so the
// vectors are identical on 32-bit builds.
var wireGolden = []struct {
	name string
	in   any
	want string
}{
	{name: "nil", in: nil, want: "c0"},
	{name: "false", in: false, want: "c2"},
	{name: "true", in: true, want: "c3"},

	{name: "posfixnum_zero", in: int(0), want: "00"},
	{name: "posfixnum_max", in: int(127), want: "7f"},
	{name: "negfixnum_min", in: int(-32), want: "e0"},
	{name: "negfixnum_minus1", in: int(-1), want: "ff"},
	{name: "int8_smallest", in: int(-33), want: "d0df"},
	{name: "int8_min", in: int(math.MinInt8), want: "d080"},
	{name: "int16_below_int8", in: int(math.MinInt8 - 1), want: "d1ff7f"},
	{name: "int16_min", in: int(math.MinInt16), want: "d18000"},
	{name: "int32_below_int16", in: int(math.MinInt16 - 1), want: "d2ffff7fff"},
	{name: "int32_min", in: int(math.MinInt32), want: "d280000000"},
	{name: "uint8_above_fixnum", in: int(128), want: "cc80"},
	{name: "uint8_max", in: int(math.MaxUint8), want: "ccff"},
	{name: "uint16_min_val", in: int(math.MaxUint8 + 1), want: "cd0100"},
	{name: "uint16_max", in: int(math.MaxUint16), want: "cdffff"},
	{name: "uint32_min_val", in: int(math.MaxUint16 + 1), want: "ce00010000"},

	{name: "typed_int8", in: int8(-1), want: "d0ff"},
	{name: "typed_int16", in: int16(-2), want: "d1fffe"},
	{name: "typed_int32", in: int32(-3), want: "d2fffffffd"},
	{name: "typed_int64", in: int64(1), want: "d30000000000000001"},
	{name: "typed_int64_min", in: int64(math.MinInt64), want: "d38000000000000000"},
	{name: "typed_uint8", in: uint8(5), want: "cc05"},
	{name: "typed_uint16", in: uint16(5), want: "cd0005"},
	{name: "typed_uint32", in: uint32(5), want: "ce00000005"},
	{name: "typed_uint32_max", in: uint32(math.MaxUint32), want: "ceffffffff"},
	{name: "typed_uint64", in: uint64(5), want: "cf0000000000000005"},
	{name: "typed_uint64_max", in: uint64(math.MaxUint64), want: "cfffffffffffffffff"},
	{name: "duration_hour", in: time.Hour, want: "d30000034630b8a000"},

	{name: "float32_pos", in: float32(1.5), want: "ca3fc00000"},
	{name: "float32_neg", in: float32(-0.5), want: "cabf000000"},
	{name: "float32_inf", in: float32(math.Inf(1)), want: "ca7f800000"},
	{name: "float64_zero", in: float64(0), want: "cb0000000000000000"},
	{name: "float64_pos", in: float64(1.5), want: "cb3ff8000000000000"},
	{name: "float64_neg", in: float64(-2.5), want: "cbc004000000000000"},
	{name: "float64_inf_neg", in: math.Inf(-1), want: "cbfff0000000000000"},

	{name: "fixstr_empty", in: "", want: "a0"},
	{name: "fixstr_one", in: "a", want: "a161"},
	{name: "fixstr_max", in: strings.Repeat("x", 31), want: "bf" + strings.Repeat("78", 31)},
	{name: "str8_min", in: strings.Repeat("x", 32), want: "d920" + strings.Repeat("78", 32)},
	{name: "str8_max", in: strings.Repeat("x", 255), want: "d9ff" + strings.Repeat("78", 255)},
	{name: "str16_min", in: strings.Repeat("x", 256), want: "da0100" + strings.Repeat("78", 256)},

	{name: "bin8_empty", in: []byte{}, want: "c400"},
	{name: "bin8_two", in: []byte{0xde, 0xad}, want: "c402dead"},
	{name: "bin8_max", in: bytes.Repeat([]byte{0xab}, 255), want: "c4ff" + strings.Repeat("ab", 255)},
	{name: "bin16_min", in: bytes.Repeat([]byte{0xab}, 256), want: "c50100" + strings.Repeat("ab", 256)},

	{name: "nil_bytes", in: []byte(nil), want: "c0"},
	{name: "nil_string_slice", in: []string(nil), want: "c0"},
	{name: "nil_map", in: map[string]int(nil), want: "c0"},

	{name: "fixarray_empty", in: []int{}, want: "90"},
	{name: "fixarray_three", in: []int{1, 2, 3}, want: "93010203"},
	{name: "fixarray_max", in: make([]int, 15), want: "9f" + strings.Repeat("00", 15)},
	{name: "array16_min", in: make([]int, 16), want: "dc0010" + strings.Repeat("00", 16)},
	{name: "fixarray_strings", in: []string{"a", "b"}, want: "92a161a162"},

	{name: "fixmap_empty", in: map[string]int{}, want: "80"},
	{name: "fixmap_one_int", in: map[string]int{"a": 1}, want: "81a16101"},
	{name: "fixmap_one_str", in: map[string]string{"k": "v"}, want: "81a16ba176"},
}

func TestWireFormatGoldenVectors(t *testing.T) {
	for _, tc := range wireGolden {
		t.Run(tc.name, func(t *testing.T) {
			data, err := msgpack.Marshal(tc.in)
			mustOK(t, err)
			if got := hex.EncodeToString(data); got != tc.want {
				t.Fatalf("marshal: got %s, want %s", got, tc.want)
			}

			if tc.in == nil {
				out := new(int)
				mustOK(t, msgpack.Unmarshal(data, &out))
				if out != nil {
					t.Fatalf("nil decode: got %#v", *out)
				}
				return
			}

			// Decode into a fresh value of the same Go type and compare.
			ptr := reflect.New(reflect.TypeOf(tc.in))
			mustOK(t, msgpack.Unmarshal(data, ptr.Interface()))
			if got := ptr.Elem().Interface(); !reflect.DeepEqual(got, tc.in) {
				t.Fatalf("roundtrip: got %#v, want %#v", got, tc.in)
			}
		})
	}
}

// canonicalIntGolden pins the smallest-width, type-losing integer
// encodings produced by EncodeInt and EncodeUint. These are the forms
// a spec-conforming encoder must emit for each boundary.
var canonicalIntGolden = []struct {
	name string
	n    int64
	want string
}{
	{name: "0", n: 0, want: "00"},
	{name: "posfix_max", n: 127, want: "7f"},
	{name: "uint8_lo", n: 128, want: "cc80"},
	{name: "uint8_hi", n: math.MaxUint8, want: "ccff"},
	{name: "uint16_lo", n: math.MaxUint8 + 1, want: "cd0100"},
	{name: "uint16_hi", n: math.MaxUint16, want: "cdffff"},
	{name: "uint32_lo", n: math.MaxUint16 + 1, want: "ce00010000"},
	{name: "uint32_hi", n: math.MaxUint32, want: "ceffffffff"},
	{name: "uint64_lo", n: math.MaxUint32 + 1, want: "cf0000000100000000"},
	{name: "negfix_min", n: -32, want: "e0"},
	{name: "neg1", n: -1, want: "ff"},
	{name: "int8_lo", n: -33, want: "d0df"},
	{name: "int8_min", n: math.MinInt8, want: "d080"},
	{name: "int16_lo", n: math.MinInt8 - 1, want: "d1ff7f"},
	{name: "int16_min", n: math.MinInt16, want: "d18000"},
	{name: "int32_lo", n: math.MinInt16 - 1, want: "d2ffff7fff"},
	{name: "int32_min", n: math.MinInt32, want: "d280000000"},
	{name: "int64_lo", n: math.MinInt32 - 1, want: "d3ffffffff7fffffff"},
	{name: "int64_min", n: math.MinInt64, want: "d38000000000000000"},
}

func TestWireFormatCanonicalInt(t *testing.T) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)

	for _, tc := range canonicalIntGolden {
		t.Run("int/"+tc.name, func(t *testing.T) {
			buf.Reset()
			mustOK(t, enc.EncodeInt(tc.n))
			if got := hex.EncodeToString(buf.Bytes()); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
			var out int64
			mustOK(t, msgpack.Unmarshal(buf.Bytes(), &out))
			mustEqual(t, out, tc.n)
		})
	}

	uintGolden := []struct {
		n    uint64
		want string
	}{
		{n: 0, want: "00"},
		{n: 127, want: "7f"},
		{n: 128, want: "cc80"},
		{n: math.MaxUint8, want: "ccff"},
		{n: math.MaxUint8 + 1, want: "cd0100"},
		{n: math.MaxUint16, want: "cdffff"},
		{n: math.MaxUint16 + 1, want: "ce00010000"},
		{n: math.MaxUint32, want: "ceffffffff"},
		{n: math.MaxUint32 + 1, want: "cf0000000100000000"},
		{n: math.MaxInt64, want: "cf7fffffffffffffff"},
		{n: math.MaxUint64, want: "cfffffffffffffffff"},
	}
	for i, tc := range uintGolden {
		t.Run("uint/"+strconv.Itoa(i), func(t *testing.T) {
			buf.Reset()
			mustOK(t, enc.EncodeUint(tc.n))
			if got := hex.EncodeToString(buf.Bytes()); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
			var out uint64
			mustOK(t, msgpack.Unmarshal(buf.Bytes(), &out))
			mustEqual(t, out, tc.n)
		})
	}
}

// TestWireFormatContainerHeaders pins the array, map, and bin length
// header bytes at every boundary between fixed, 16-bit, and 32-bit forms.
func TestWireFormatContainerHeaders(t *testing.T) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)

	arrayCases := []struct {
		n    int
		want string
	}{
		{0, "90"},
		{15, "9f"},
		{16, "dc0010"},
		{math.MaxUint16, "dcffff"},
		{math.MaxUint16 + 1, "dd00010000"},
	}
	for _, tc := range arrayCases {
		buf.Reset()
		mustOK(t, enc.EncodeArrayLen(tc.n))
		if got := hex.EncodeToString(buf.Bytes()); got != tc.want {
			t.Fatalf("EncodeArrayLen(%d): got %s, want %s", tc.n, got, tc.want)
		}
		// Decode over a Len-less reader: a bare header on a sized reader
		// is correctly fail-fast rejected by the remaining-input guard.
		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(buf.Bytes())})
		n, err := dec.DecodeArrayLen()
		mustOK(t, err)
		mustEqual(t, n, tc.n)
	}

	mapCases := []struct {
		n    int
		want string
	}{
		{0, "80"},
		{15, "8f"},
		{16, "de0010"},
		{math.MaxUint16, "deffff"},
		{math.MaxUint16 + 1, "df00010000"},
	}
	for _, tc := range mapCases {
		buf.Reset()
		mustOK(t, enc.EncodeMapLen(tc.n))
		if got := hex.EncodeToString(buf.Bytes()); got != tc.want {
			t.Fatalf("EncodeMapLen(%d): got %s, want %s", tc.n, got, tc.want)
		}
		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(buf.Bytes())})
		n, err := dec.DecodeMapLen()
		mustOK(t, err)
		mustEqual(t, n, tc.n)
	}

	binCases := []struct {
		n    int
		want string
	}{
		{0, "c400"},
		{math.MaxUint8, "c4ff"},
		{math.MaxUint8 + 1, "c50100"},
		{math.MaxUint16, "c5ffff"},
		{math.MaxUint16 + 1, "c600010000"},
	}
	for _, tc := range binCases {
		buf.Reset()
		mustOK(t, enc.EncodeBytesLen(tc.n))
		if got := hex.EncodeToString(buf.Bytes()); got != tc.want {
			t.Fatalf("EncodeBytesLen(%d): got %s, want %s", tc.n, got, tc.want)
		}
	}
}

// TestWireFormatExtHeaders pins every ext header form: fixext1/2/4/8/16
// for the fixed payload sizes, and ext8/16/32 for everything else.
func TestWireFormatExtHeaders(t *testing.T) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)

	cases := []struct {
		extLen int
		want   string
	}{
		{1, "d4"},
		{2, "d5"},
		{4, "d6"},
		{8, "d7"},
		{16, "d8"},
		{0, "c700"},
		{3, "c703"},
		{math.MaxUint8, "c7ff"},
		{math.MaxUint8 + 1, "c80100"},
		{math.MaxUint16, "c8ffff"},
		{math.MaxUint16 + 1, "c900010000"},
	}
	for _, tc := range cases {
		buf.Reset()
		mustOK(t, enc.EncodeExtHeader(7, tc.extLen))
		want := tc.want + "07"
		if got := hex.EncodeToString(buf.Bytes()); got != want {
			t.Fatalf("EncodeExtHeader(7, %d): got %s, want %s", tc.extLen, got, want)
		}

		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(buf.Bytes())})
		extID, extLen, err := dec.DecodeExtHeader()
		mustOK(t, err)
		mustEqual(t, extID, int8(7))
		mustEqual(t, extLen, tc.extLen)
	}
}

// TestWireFormatTimestamps pins the three timestamp ext layouts from the
// spec: fixext4 (32-bit seconds), fixext8 (30-bit nanos + 34-bit
// seconds), and ext8/12 (32-bit nanos + 64-bit signed seconds).
func TestWireFormatTimestamps(t *testing.T) {
	cases := []struct {
		name string
		in   time.Time
		want string
	}{
		{name: "epoch", in: time.Unix(0, 0), want: "d6ff00000000"},
		{name: "fixext8", in: time.Unix(1, 1), want: "d7ff0000000400000001"},
		{name: "fixext8_nanos", in: time.Unix(0, 999999999), want: "d7ffee6b27fc00000000"},
		{name: "zero_time", in: time.Time{}, want: "c70cff00000000fffffff1886e0900"},
		{name: "negative_seconds", in: time.Unix(-1, 0), want: "c70cff00000000ffffffffffffffff"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := msgpack.Marshal(tc.in)
			mustOK(t, err)
			if got := hex.EncodeToString(data); got != tc.want {
				t.Fatalf("marshal: got %s, want %s", got, tc.want)
			}

			var out time.Time
			mustOK(t, msgpack.Unmarshal(data, &out))
			mustTrue(t, out.Equal(tc.in), "decoded time mismatch")

			var v any
			mustOK(t, msgpack.Unmarshal(data, &v))
			tm, ok := v.(time.Time)
			mustTrue(t, ok, "interface decode produced non-time value")
			mustTrue(t, tm.Equal(tc.in), "interface decoded time mismatch")
		})
	}
}

// TestWireFormatSortedMap pins a fully deterministic map encoding: with
// SetSortMapKeys the key order is sorted on the wire.
func TestWireFormatSortedMap(t *testing.T) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	enc.SetSortMapKeys(true)

	mustOK(t, enc.Encode(map[string]bool{"c": true, "a": true, "b": false}))
	if got := hex.EncodeToString(buf.Bytes()); got != "83a161c3a162c2a163c3" {
		t.Fatalf("got %s, want %s", got, "83a161c3a162c2a163c3")
	}
}
