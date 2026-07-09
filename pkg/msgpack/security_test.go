package msgpack_test

import (
	"bytes"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"quad4/msgpack/v5/pkg/msgpack"
)

func TestLengthPrefixOverflowGuards(t *testing.T) {
	t.Run("bytes_len", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xc6, 0xff, 0xff, 0xff, 0xff}))
		n, err := dec.DecodeBytesLen()
		assertUint32LenBehavior(t, "bytes length", n, err)
	})

	t.Run("array_len", func(t *testing.T) {
		// Header-only array32 with a max length must fail fast: there are
		// zero remaining payload bytes, so the claimed length is impossible.
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xdd, 0xff, 0xff, 0xff, 0xff}))
		n, err := dec.DecodeArrayLen()
		assertOversizedContainerRejected(t, "array length", n, err)
	})

	t.Run("map_len", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xdf, 0xff, 0xff, 0xff, 0xff}))
		n, err := dec.DecodeMapLen()
		assertOversizedContainerRejected(t, "map length", n, err)
	})

	t.Run("ext_len", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xc9, 0xff, 0xff, 0xff, 0xff, 0x01}))
		_, n, err := dec.DecodeExtHeader()
		assertUint32LenBehavior(t, "ext length", n, err)
	})
}

func TestRejectForgedArray32DoesNotAllocateGigabytes(t *testing.T) {
	// Nested array32 claiming ~3.7e9 elements with only ~2KB of trailing
	// filler previously forced Unmarshal into []any to allocate multi-GB
	// before EOF. Found via fuzzing in reticulum-go. The remaining-input
	// guard must reject this immediately.
	assertForgedContainerDoesNotAllocate(t, reticulumGoForgedArray32Payload(), "array")
}

func TestRejectForgedMap32DoesNotAllocateGigabytes(t *testing.T) {
	// Header-only map32 with a max length must fail before allocating a
	// multi-gigabyte map for missing key/value pairs.
	assertForgedContainerDoesNotAllocate(t, []byte{0xdf, 0xff, 0xff, 0xff, 0xff}, "map")
}

func TestRejectOversizedContainerHeaders(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		kind string
		call func(*msgpack.Decoder) (int, error)
	}{
		{
			name: "array16_max_no_payload",
			data: []byte{0xdc, 0xff, 0xff},
			kind: "array",
			call: func(d *msgpack.Decoder) (int, error) { return d.DecodeArrayLen() },
		},
		{
			name: "array32_max_no_payload",
			data: []byte{0xdd, 0xff, 0xff, 0xff, 0xff},
			kind: "array",
			call: func(d *msgpack.Decoder) (int, error) { return d.DecodeArrayLen() },
		},
		{
			name: "map16_max_no_payload",
			data: []byte{0xde, 0xff, 0xff},
			kind: "map",
			call: func(d *msgpack.Decoder) (int, error) { return d.DecodeMapLen() },
		},
		{
			name: "map32_max_no_payload",
			data: []byte{0xdf, 0xff, 0xff, 0xff, 0xff},
			kind: "map",
			call: func(d *msgpack.Decoder) (int, error) { return d.DecodeMapLen() },
		},
		{
			name: "fixarray_truncated",
			// fixarray 4 with only one remaining byte cannot hold 4 elements.
			data: []byte{0x94, 0x01},
			kind: "array",
			call: func(d *msgpack.Decoder) (int, error) { return d.DecodeArrayLen() },
		},
		{
			name: "fixmap_truncated",
			// fixmap 2 needs at least 4 bytes of key/value payload; only one remains.
			data: []byte{0x82, 0xa1},
			kind: "map",
			call: func(d *msgpack.Decoder) (int, error) { return d.DecodeMapLen() },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dec := msgpack.NewDecoder(bytes.NewReader(tc.data))
			n, err := tc.call(dec)
			assertOversizedContainerRejected(t, tc.kind+" length", n, err)
		})
	}
}

func TestRejectOversizedContainerViaSkip(t *testing.T) {
	// Skip walks array/map lengths the same way Decode does; forged
	// headers must fail fast here too (Query and struct field skipping).
	cases := [][]byte{
		{0xdd, 0xff, 0xff, 0xff, 0xff},
		{0xdf, 0xff, 0xff, 0xff, 0xff},
		{0xdc, 0xff, 0xff},
		{0xde, 0xff, 0xff},
		reticulumGoForgedArray32Payload(),
	}
	for i, data := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			dec := msgpack.NewDecoder(bytes.NewReader(data))
			err := dec.Skip()
			if err == nil {
				t.Fatal("expected Skip to reject oversized container")
			}
			if !strings.Contains(err.Error(), "exceeds remaining input") {
				t.Fatalf("expected remaining-input error, got %v", err)
			}
		})
	}
}

// reticulumGoForgedArray32Payload is the shape that OOMed fuzz workers in
// reticulum-go: a short fixarray whose third element is an array32 header
// claiming ~3.7e9 elements, followed by ~2KB of 0xdd filler.
func reticulumGoForgedArray32Payload() []byte {
	payload := make([]byte, 0, 2200)
	payload = append(payload, 0x9a)       // fixarray 10
	payload = append(payload, 0xd6, 0xff) // fixext4 type=-1
	payload = append(payload, '0', '0', '0', '0')
	payload = append(payload, 'E') // positive fixint
	payload = append(payload, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd)
	for len(payload) < 2150 {
		payload = append(payload, 0xdd)
	}
	return payload
}

func assertForgedContainerDoesNotAllocate(t *testing.T, payload []byte, kind string) {
	t.Helper()
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	var out any
	err := msgpack.Unmarshal(payload, &out)
	runtime.ReadMemStats(&m2)
	if err == nil {
		t.Fatalf("expected forged %s to fail", kind)
	}
	if !strings.Contains(err.Error(), "exceeds remaining input") {
		t.Fatalf("expected remaining-input error, got %v", err)
	}
	alloc := m2.TotalAlloc - m1.TotalAlloc
	const maxAlloc = 32 << 20 // 32 MiB ceiling; previously multi-GiB
	if alloc > maxAlloc {
		t.Fatalf("forged %s allocated %d bytes, want <= %d", kind, alloc, maxAlloc)
	}
}

func TestDecodeDepthLimitGuards(t *testing.T) {
	const (
		limit = 64
		depth = 256
	)
	data := nestedArrayBytes(depth)

	t.Run("decode_interface_limit", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		dec.SetDecodeDepthLimit(limit)

		var out any
		err := dec.Decode(&out)
		if err == nil || !strings.Contains(err.Error(), "decode nesting depth exceeds limit") {
			t.Fatalf("expected depth-limit error, got %v", err)
		}
	})

	t.Run("skip_limit", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		dec.SetDecodeDepthLimit(limit)

		err := dec.Skip()
		if err == nil || !strings.Contains(err.Error(), "decode nesting depth exceeds limit") {
			t.Fatalf("expected depth-limit error from Skip, got %v", err)
		}
	})

	t.Run("limit_override_allows_legit_depth", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		dec.SetDecodeDepthLimit(depth + 16)

		var out any
		if err := dec.Decode(&out); err != nil {
			t.Fatalf("decode with higher limit failed: %v", err)
		}
	})
}

func assertUint32LenBehavior(t *testing.T, hint string, n int, err error) {
	t.Helper()
	if strconv.IntSize == 32 {
		if err == nil || !strings.Contains(err.Error(), "overflows int") {
			t.Fatalf("%s: expected overflow error on 32-bit, got n=%d err=%v", hint, n, err)
		}
		return
	}

	if err != nil {
		t.Fatalf("%s: expected success on 64-bit, got err=%v", hint, err)
	}
	if uint64(n) != uint64(^uint32(0)) {
		t.Fatalf("%s: unexpected length: got %d", hint, n)
	}
}

func assertOversizedContainerRejected(t *testing.T, hint string, n int, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected remaining-input rejection, got n=%d", hint, n)
	}
	if !strings.Contains(err.Error(), "exceeds remaining input") {
		t.Fatalf("%s: expected remaining-input error, got %v", hint, err)
	}
	if n != 0 {
		t.Fatalf("%s: expected zero length on rejection, got %d", hint, n)
	}
}

func nestedArrayBytes(depth int) []byte {
	b := make([]byte, 0, depth+1)
	for range depth {
		b = append(b, 0x91)
	}
	b = append(b, 0xc0)
	return b
}
