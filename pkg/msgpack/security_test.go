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
		assertOversizedBytesRejected(t, "bytes length", n, err)
	})

	t.Run("array_len", func(t *testing.T) {
		// Header-only array32 with a max length must fail fast when there
		// are zero remaining payload bytes.
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
		assertOversizedBytesRejected(t, "ext length", n, err)
	})
}

func TestRejectForgedArray32DoesNotAllocateGigabytes(t *testing.T) {
	// Nested array32 claiming about 3.7e9 elements with only about 2KB of
	// trailing filler must be rejected before Unmarshal into any allocates
	// multi-gigabyte backing storage.
	assertForgedContainerDoesNotAllocate(t, reticulumGoForgedArray32Payload(), "array")
}

func TestRejectForgedMap32DoesNotAllocateGigabytes(t *testing.T) {
	// Header-only map32 with a max length must fail before allocating a
	// multi-gigabyte map for missing key and value pairs.
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
			// fixmap 2 needs at least 4 bytes of key/value payload. Only one remains.
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
	// Skip walks array and map lengths the same way Decode does. Oversized
	// headers must fail fast here too for Query and struct field skipping.
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
			if !isFailFastContainerError(err) {
				t.Fatalf("expected fail-fast container error, got %v", err)
			}
		})
	}
}

// reticulumGoForgedArray32Payload builds a short fixarray whose third
// element is an array32 header claiming about 3.7e9 elements, followed by
// about 2KB of 0xdd filler. This is a regression seed for the remaining-input
// length guard.
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
	if !isFailFastContainerError(err) {
		t.Fatalf("expected fail-fast container error, got %v", err)
	}
	alloc := m2.TotalAlloc - m1.TotalAlloc
	const maxAlloc = 32 << 20 // 32 MiB ceiling for fail-fast rejection
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

func assertOversizedBytesRejected(t *testing.T, hint string, n int, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected fail-fast rejection, got n=%d", hint, n)
	}
	msg := err.Error()
	if !strings.Contains(msg, "exceeds remaining input") && !strings.Contains(msg, "overflows int") {
		t.Fatalf("%s: expected remaining-input or overflow error, got %v", hint, err)
	}
	if n != 0 {
		t.Fatalf("%s: expected zero length on rejection, got %d", hint, n)
	}
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
		t.Fatalf("%s: expected fail-fast rejection, got n=%d", hint, n)
	}
	if !isFailFastContainerError(err) {
		t.Fatalf("%s: expected fail-fast container error, got %v", hint, err)
	}
	if n != 0 {
		t.Fatalf("%s: expected zero length on rejection, got %d", hint, n)
	}
}

// isFailFastContainerError reports whether err is a fast rejection of an
// oversized array or map length. On 64-bit this is usually the remaining-input
// guard or the soft alloc ceiling for readers without Len. On 32-bit, lengths
// above math.MaxInt32 are rejected earlier by uint32ToInt.
func isFailFastContainerError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "exceeds remaining input") ||
		strings.Contains(msg, "exceeds decode limit") ||
		strings.Contains(msg, "overflows int")
}

func nestedArrayBytes(depth int) []byte {
	b := make([]byte, 0, depth+1)
	for range depth {
		b = append(b, 0x91)
	}
	b = append(b, 0xc0)
	return b
}
