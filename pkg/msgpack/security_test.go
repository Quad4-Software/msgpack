package msgpack_test

import (
	"bytes"
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
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xdd, 0xff, 0xff, 0xff, 0xff}))
		n, err := dec.DecodeArrayLen()
		assertUint32LenBehavior(t, "array length", n, err)
	})

	t.Run("map_len", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xdf, 0xff, 0xff, 0xff, 0xff}))
		n, err := dec.DecodeMapLen()
		assertUint32LenBehavior(t, "map length", n, err)
	})

	t.Run("ext_len", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xc9, 0xff, 0xff, 0xff, 0xff, 0x01}))
		_, n, err := dec.DecodeExtHeader()
		assertUint32LenBehavior(t, "ext length", n, err)
	})
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

func nestedArrayBytes(depth int) []byte {
	b := make([]byte, 0, depth+1)
	for range depth {
		b = append(b, 0x91)
	}
	b = append(b, 0xc0)
	return b
}
