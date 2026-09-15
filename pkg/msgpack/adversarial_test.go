package msgpack_test

// Adversarial decode tests: malformed, truncated, forged, and
// out-of-spec inputs must produce errors, never panics or oversized
// allocations. Also documents behaviors that are legal-but-surprising:
// trailing bytes are left unread and duplicate map keys are last-wins.

import (
	"bytes"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/Quad4-Software/msgpack/v5/pkg/msgpack"
)

// TestReservedCodeC1Rejected pins the handling of 0xc1, the one byte the
// spec reserves and that must never appear on the wire.
func TestReservedCodeC1Rejected(t *testing.T) {
	t.Run("interface", func(t *testing.T) {
		var out any
		err := msgpack.Unmarshal([]byte{0xc1}, &out)
		mustErrorString(t, err, "msgpack: unknown code c1 decoding interface{}")
	})

	t.Run("skip", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xc1}))
		mustErrorString(t, dec.Skip(), "msgpack: unknown code c1")
	})

	t.Run("typed", func(t *testing.T) {
		var s string
		err := msgpack.Unmarshal([]byte{0xc1}, &s)
		mustErrorString(t, err, "msgpack: invalid code=c1 decoding string/bytes length")
	})

	t.Run("nested", func(t *testing.T) {
		var out []any
		err := msgpack.Unmarshal([]byte{0x92, 0x01, 0xc1}, &out)
		if err == nil {
			t.Fatalf("expected error decoding array containing 0xc1, got %#v", out)
		}
	})
}

// TestTruncatedInputsRejected feeds every proper prefix of a well-formed
// nested payload to Unmarshal. Each one must return an error; none may
// panic or silently decode a partial value.
func TestTruncatedInputsRejected(t *testing.T) {
	full, err := msgpack.Marshal(map[string]any{
		"k": []any{int64(1), "two", []byte{3}},
		"s": "hello",
	})
	mustOK(t, err)
	if len(full) < 8 {
		t.Fatalf("test payload too small: % x", full)
	}

	for i := range len(full) {
		var out any
		if err := msgpack.Unmarshal(full[:i], &out); err == nil {
			t.Fatalf("prefix len=%d decoded without error: %#v (full=%x)", i, out, full)
		}
	}
}

// TestTruncatedHeadersRejected covers inputs that end inside a
// multi-byte header or inside a fixed-size payload.
func TestTruncatedHeadersRejected(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		call func(*msgpack.Decoder) error
	}{
		{"str32_partial_len", []byte{0xdb, 0x00}, func(d *msgpack.Decoder) error { _, err := d.DecodeString(); return err }},
		{"str16_short_payload", []byte{0xda, 0x00, 0x0a, 'h'}, func(d *msgpack.Decoder) error { _, err := d.DecodeString(); return err }},
		{"bin16_partial_len", []byte{0xc5, 0x00}, func(d *msgpack.Decoder) error { _, err := d.DecodeBytes(); return err }},
		{"bin8_short_payload", []byte{0xc4, 0x05, 0x01}, func(d *msgpack.Decoder) error { _, err := d.DecodeBytes(); return err }},
		{"array32_partial_len", []byte{0xdd, 0x00, 0x00}, func(d *msgpack.Decoder) error { _, err := d.DecodeArrayLen(); return err }},
		{"map16_partial_len", []byte{0xde, 0x00}, func(d *msgpack.Decoder) error { _, err := d.DecodeMapLen(); return err }},
		{"ext32_no_type_byte", []byte{0xc9, 0x00, 0x00, 0x00, 0x01}, func(d *msgpack.Decoder) error { _, _, err := d.DecodeExtHeader(); return err }},
		{"fixext4_short_payload", []byte{0xd6, 0xff, 0x01, 0x02}, func(d *msgpack.Decoder) error { return d.Skip() }},
		{"ext8_short_payload", []byte{0xc7, 0x05, 0x01, 'a', 'b'}, func(d *msgpack.Decoder) error { return d.Skip() }},
		{"double_short", []byte{0xcb, 0x3f}, func(d *msgpack.Decoder) error { _, err := d.DecodeFloat64(); return err }},
		{"uint16_short", []byte{0xcd, 0x01}, func(d *msgpack.Decoder) error { _, err := d.DecodeUint64(); return err }},
		{"int64_short", []byte{0xd3, 0x01, 0x02}, func(d *msgpack.Decoder) error { _, err := d.DecodeInt64(); return err }},
		{"empty", nil, func(d *msgpack.Decoder) error { var v any; return d.Decode(&v) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dec := msgpack.NewDecoder(bytes.NewReader(tc.data))
			if err := tc.call(dec); err == nil {
				t.Fatalf("expected error for truncated input %x", tc.data)
			}
		})
	}
}

// TestUnmarshalIgnoresTrailingGarbage documents that Unmarshal decodes
// the first complete value and leaves trailing bytes unread. Callers
// that need strict framing must drive a Decoder themselves.
func TestUnmarshalIgnoresTrailingGarbage(t *testing.T) {
	data, err := msgpack.Marshal(42)
	mustOK(t, err)

	trailing := append(append([]byte{}, data...), 0xc1, 0xff, 0xff)
	var n int
	mustOK(t, msgpack.Unmarshal(trailing, &n))
	mustEqual(t, n, 42)
}

// TestDecoderSequentialValues documents stream decoding: a Decoder reads
// one value at a time and reports the next wire code, including errors
// for garbage between values.
func TestDecoderSequentialValues(t *testing.T) {
	a, err := msgpack.Marshal("first")
	mustOK(t, err)
	b, err := msgpack.Marshal(7)
	mustOK(t, err)

	stream := append(append(append([]byte{}, a...), 0xc1), b...)
	dec := msgpack.NewDecoder(bytes.NewReader(stream))

	var s string
	mustOK(t, dec.Decode(&s))
	mustEqual(t, s, "first")

	// The 0xc1 byte between the two values surfaces on the next read.
	var mid any
	if err := dec.Decode(&mid); err == nil {
		t.Fatalf("expected error for interleaved 0xc1, got %#v", mid)
	}

	var n int
	mustOK(t, dec.Decode(&n))
	mustEqual(t, n, 7)

	var end any
	mustEqual(t, dec.Decode(&end), io.EOF)
}

// TestDuplicateMapKeysLastWins documents duplicate-key behavior: the
// decoder applies last-write-wins, matching Go map assignment
// semantics. No dedup or size-mismatch error is produced.
func TestDuplicateMapKeysLastWins(t *testing.T) {
	// fixmap 2 with the same key twice and different values.
	data := []byte{0x82, 0xa1, 'a', 0x01, 0xa1, 'a', 0x02}

	var m map[string]any
	mustOK(t, msgpack.Unmarshal(data, &m))
	mustEqual(t, len(m), 1)
	mustEqual(t, m["a"], any(int8(2)))

	var mt map[string]int
	mustOK(t, msgpack.Unmarshal(data, &mt))
	mustEqual(t, mt["a"], 2)

	var mu map[any]any
	dec := msgpack.NewDecoder(bytes.NewReader(data))
	dec.SetMapDecoder(func(d *msgpack.Decoder) (any, error) {
		return d.DecodeUntypedMap()
	})
	mustOK(t, dec.Decode(&mu))
	mustEqual(t, mu[any("a")], any(int8(2)))
}

// TestDefaultDecodeDepthLimit pins the default 10000-level nesting cap:
// nested(d) arrays into interface{} require depth d+2 (one per array
// element plus the enclosing DecodeValue and DecodeInterface frames), so
// 9998 levels succeed and 9999 fail.
func TestDefaultDecodeDepthLimit(t *testing.T) {
	t.Run("rejects_over_default", func(t *testing.T) {
		var out any
		err := msgpack.Unmarshal(nestedArrayBytes(9999), &out)
		mustErrorString(t, err, "msgpack: decode nesting depth exceeds limit=10000")
	})

	t.Run("accepts_under_default", func(t *testing.T) {
		var out any
		mustOK(t, msgpack.Unmarshal(nestedArrayBytes(9998), &out))
	})

	t.Run("skip_rejects_over_default", func(t *testing.T) {
		// Skip enters one depth frame per nesting level while the
		// interface decode path enters two, so the Skip boundary sits
		// one level deeper than the Decode boundary.
		dec := msgpack.NewDecoder(bytes.NewReader(nestedArrayBytes(10000)))
		mustErrorString(t, dec.Skip(), "msgpack: decode nesting depth exceeds limit=10000")
	})

	t.Run("nonpositive_restores_default", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader(nestedArrayBytes(200)))
		dec.SetDecodeDepthLimit(4)
		var out any
		if err := dec.Decode(&out); err == nil {
			t.Fatal("expected depth error at limit=4")
		}
		for _, reset := range []int{0, -1} {
			dec.Reset(bytes.NewReader(nestedArrayBytes(200)))
			dec.SetDecodeDepthLimit(reset)
			var fresh any
			if err := dec.Decode(&fresh); err != nil {
				t.Fatalf("SetDecodeDepthLimit(%d): decode at depth 200 failed: %v", reset, err)
			}
		}
	})
}

// TestDisableAllocLimitRestore is the regression test for the
// disableAllocLimitFlag bit-check fix. The flag is bit 3 of the flag
// word; the previous code compared the masked value against the literal
// 1, which silently broke the check in both directions depending on
// which flags were set. Each subtest would fail on the old code path.
func TestDisableAllocLimitRestore(t *testing.T) {
	// array32 header claiming two million elements with no payload.
	forged := []byte{0xdd, 0x00, 0x1e, 0x84, 0x80}

	t.Run("default_rejects_forged_header", func(t *testing.T) {
		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(forged)})
		var out []any
		err := dec.Decode(&out)
		mustErrorString(t, err, "msgpack: array length 2000000 exceeds decode limit (1000000)")
	})

	t.Run("disable_bypasses_soft_ceiling", func(t *testing.T) {
		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(forged)})
		dec.DisableAllocLimit(true)
		var out []any
		err := dec.Decode(&out)
		if err == nil {
			t.Fatal("expected EOF decoding forged array32 with no payload")
		}
		if strings.Contains(err.Error(), "exceeds decode limit") {
			t.Fatalf("DisableAllocLimit(true) did not restore unbounded decode: %v", err)
		}
	})

	t.Run("disable_works_with_other_flags_set", func(t *testing.T) {
		// The regression: comparing flags against the literal 1 breaks
		// once the flag word holds more than one bit.
		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(forged)})
		dec.UseLooseInterfaceDecoding(true)
		dec.DisallowUnknownFields(true)
		dec.DisableAllocLimit(true)
		var out []any
		err := dec.Decode(&out)
		if err == nil {
			t.Fatal("expected EOF decoding forged array32 with no payload")
		}
		if strings.Contains(err.Error(), "exceeds decode limit") {
			t.Fatalf("DisableAllocLimit ignored when combined with other flags: %v", err)
		}
	})

	t.Run("legit_over_ceiling_decodes_when_disabled", func(t *testing.T) {
		const n = 1_000_001 // one past sliceAllocLimit
		payload := append([]byte{0xdd, 0x00, 0x0f, 0x42, 0x41}, bytes.Repeat([]byte{0x00}, n)...)

		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(payload)})
		var out []any
		err := dec.Decode(&out)
		if err == nil || !strings.Contains(err.Error(), "exceeds decode limit") {
			t.Fatalf("expected soft-ceiling rejection for over-limit array, got %v", err)
		}

		dec = msgpack.NewDecoder(limitedReader{bytes.NewReader(payload)})
		dec.DisableAllocLimit(true)
		out = nil
		mustOK(t, dec.Decode(&out))
		mustEqual(t, len(out), n)
	})

	t.Run("growth_still_allowed_with_known_len", func(t *testing.T) {
		// With a sized reader the remaining-input guard passes for a
		// legit over-ceiling array and decodeSlice grows the result via
		// append as elements arrive.
		const n = 1_000_001
		payload := append([]byte{0xdd, 0x00, 0x0f, 0x42, 0x41}, bytes.Repeat([]byte{0x00}, n)...)
		var out []any
		mustOK(t, msgpack.Unmarshal(payload, &out))
		mustEqual(t, len(out), n)
	})
}

// TestAllocLimitClampsByteGrowth verifies the two halves of the
// bytesAllocLimit claim: a forged bin32 length cannot force a giant
// up-front allocation over a Len-less reader, while a legit payload
// larger than the cap still decodes by growing in bounded chunks.
func TestAllocLimitClampsByteGrowth(t *testing.T) {
	t.Run("forged_bin32_bounded_alloc", func(t *testing.T) {
		// bin32 claiming 64 MiB with only 1 KiB of payload.
		payload := append([]byte{0xc6, 0x04, 0x00, 0x00, 0x00}, make([]byte, 1024)...)

		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(payload)})
		var out []byte
		err := dec.Decode(&out)
		runtime.ReadMemStats(&m2)
		if err == nil {
			t.Fatal("expected error decoding forged bin32")
		}
		// The grow-bounded reader must not allocate the claimed 64 MiB.
		const maxAlloc = 8 << 20
		if alloc := m2.TotalAlloc - m1.TotalAlloc; alloc > maxAlloc {
			t.Fatalf("forged bin32 allocated %d bytes, want <= %d", alloc, maxAlloc)
		}
	})

	t.Run("legit_over_cap_decodes", func(t *testing.T) {
		// 2 MiB of real payload exceeds the 1 MiB clamp; the decoder
		// must grow incrementally and return all of it.
		src := make([]byte, 2<<20)
		for i := range src {
			src[i] = byte(i)
		}
		payload, err := msgpack.Marshal(src)
		mustOK(t, err)

		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(payload)})
		var out []byte
		mustOK(t, dec.Decode(&out))
		mustBytesEqual(t, out, src)
	})

	t.Run("typed_slice_grows_past_cap", func(t *testing.T) {
		// decodeSliceValue grows by at most sliceAllocLimit per step.
		// A legit array just over the cap must still complete.
		const n = 1_000_001
		payload := append([]byte{0xdd, 0x00, 0x0f, 0x42, 0x41}, bytes.Repeat([]byte{0x00}, n)...)
		var out []int
		mustOK(t, msgpack.Unmarshal(payload, &out))
		mustEqual(t, len(out), n)
	})
}

// TestSkipExt32MaxLenNoOverflow is the regression test for the skipExt
// n+1 int overflow. On 32-bit builds an ext32 header claiming
// math.MaxInt32 bytes used to panic on a negative slice bound; on any
// platform it must return an error instead.
func TestSkipExt32MaxLenNoOverflow(t *testing.T) {
	payload := []byte{0xc9, 0x7f, 0xff, 0xff, 0xff, 0x01}
	dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(payload)})
	if err := dec.Skip(); err == nil {
		t.Fatal("expected error skipping ext32 with MaxInt32 length")
	}
}

// TestDepthAndAllocLimitInterplay pins which guard fires first when a
// forged container header sits inside a nested structure: the depth
// check runs when entering the element, before the innermost header is
// read.
func TestDepthAndAllocLimitInterplay(t *testing.T) {
	// Three nested fixarrays wrapping a forged array32 header. The
	// claimed length is MaxInt32 so it stays representable on 32-bit
	// builds and both length guards can be reached there.
	data := []byte{0x91, 0x91, 0x91, 0xdd, 0x7f, 0xff, 0xff, 0xff}

	t.Run("depth_fires_first", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader(data))
		dec.SetDecodeDepthLimit(3)
		var out any
		err := dec.Decode(&out)
		mustErrorString(t, err, "msgpack: decode nesting depth exceeds limit=3")
	})

	t.Run("sized_reader_uses_remaining_input", func(t *testing.T) {
		var out any
		err := msgpack.Unmarshal(data, &out)
		mustErrorString(t, err, "msgpack: array length 2147483647 exceeds remaining input (0 bytes)")
	})

	t.Run("lenless_reader_uses_soft_ceiling", func(t *testing.T) {
		dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(data)})
		var out any
		err := dec.Decode(&out)
		mustErrorString(t, err, "msgpack: array length 2147483647 exceeds decode limit (1000000)")
	})
}

// TestNilReaderDecodeReturnsEOF covers the degenerate case of a Decoder
// with no underlying reader: every entry point must return io.EOF
// rather than panic on a nil interface method call.
func TestNilReaderDecodeReturnsEOF(t *testing.T) {
	t.Run("new_decoder_nil", func(t *testing.T) {
		dec := msgpack.NewDecoder(nil)
		var out any
		mustEqual(t, dec.Decode(&out), io.EOF)
		mustEqual(t, dec.Skip(), io.EOF)
		_, err := dec.PeekCode()
		mustEqual(t, err, io.EOF)
		mustEqual(t, dec.ReadFull(make([]byte, 4)), io.EOF)
	})

	t.Run("reset_nil", func(t *testing.T) {
		dec := msgpack.NewDecoder(bytes.NewReader([]byte{0x01}))
		var n int
		mustOK(t, dec.Decode(&n))
		dec.Reset(nil)
		mustEqual(t, dec.Decode(&n), io.EOF)
	})
}
