package msgpack

// White-box regression tests for internals that are not observable
// through the public API: the decode-recording buffer (d.rec) and the
// pooled Encoder/Decoder scratch-buffer capping. See decode.go,
// decode_value.go, and encode.go for the corresponding fixes.

import (
	"bytes"
	"testing"
)

// truncatedExtHeader parses as an Ext32 with a huge declared length but
// runs out of input before the mandatory ext-type byte, so Skip fails
// partway through recording.
var truncatedExtHeader = []byte{0xc9, 0x00, 0x00, 0x00, 0x01}

func TestDecodeRawClearsRecOnError(t *testing.T) {
	dec := NewDecoder(bytes.NewReader(truncatedExtHeader))
	if _, err := dec.DecodeRaw(); err == nil {
		t.Fatalf("expected error from truncated ext header")
	}
	if dec.rec != nil {
		t.Fatalf("dec.rec leaked after DecodeRaw error: len=%d cap=%d", len(dec.rec), cap(dec.rec))
	}
}

func TestDecodeRawClearsRecOnSuccess(t *testing.T) {
	data, err := Marshal("hello")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	dec := NewDecoder(bytes.NewReader(data))
	msg, err := dec.DecodeRaw()
	if err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if !bytes.Equal(msg, data) {
		t.Fatalf("raw mismatch: got % x want % x", msg, data)
	}
	if dec.rec != nil {
		t.Fatalf("dec.rec not cleared after successful DecodeRaw")
	}
}

// TestDecodeRawErrorDoesNotPoisonSubsequentReads confirms the fixed
// behavior end to end: after a failed DecodeRaw, reusing the same
// Decoder for an unrelated, well-formed decode works normally and does
// not silently keep recording into the abandoned buffer.
func TestDecodeRawErrorDoesNotPoisonSubsequentReads(t *testing.T) {
	data, err := Marshal("hello")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// 0xc1 is a reserved code that is never valid on the wire; Skip
	// rejects it immediately after consuming exactly that one byte,
	// regardless of what data follows, so concatenating a well-formed
	// payload after it exercises the "already failed, now reused"
	// scenario deterministically.
	badCode := []byte{0xc1}
	dec := NewDecoder(bytes.NewReader(append(bytes.Clone(badCode), data...)))
	if _, err := dec.DecodeRaw(); err == nil {
		t.Fatalf("expected error from reserved code 0xc1")
	}

	var s string
	if err := dec.Decode(&s); err != nil {
		t.Fatalf("decode after prior error: %v", err)
	}
	if s != "hello" {
		t.Fatalf("got %q, want %q", s, "hello")
	}
	if dec.rec != nil {
		t.Fatalf("dec.rec leaked into unrelated subsequent decode")
	}
}

type recLeakUnmarshaler struct{ s string }

func (u *recLeakUnmarshaler) UnmarshalMsgpack(b []byte) error {
	u.s = string(b)
	return nil
}

func TestUnmarshalValueClearsRecOnError(t *testing.T) {
	dec := NewDecoder(bytes.NewReader(truncatedExtHeader))
	var u recLeakUnmarshaler
	if err := dec.Decode(&u); err == nil {
		t.Fatalf("expected error from truncated ext header")
	}
	if dec.rec != nil {
		t.Fatalf("dec.rec leaked after unmarshalValue error: len=%d cap=%d", len(dec.rec), cap(dec.rec))
	}
}

func TestPutDecoderDropsOversizedBuffers(t *testing.T) {
	dec := NewDecoder(nil)
	dec.buf = make([]byte, 0, maxPooledBufSize+1)
	dec.rec = make([]byte, 0, maxPooledBufSize+1)

	PutDecoder(dec)

	if dec.buf != nil {
		t.Fatalf("expected oversized buf to be dropped, cap=%d", cap(dec.buf))
	}
	if dec.rec != nil {
		t.Fatalf("expected oversized rec to be dropped, cap=%d", cap(dec.rec))
	}
}

// TestPooledDecoderDoesNotRetainHugePayloadCapacity is an end-to-end
// check that a Decoder pulled from the package pool, used to decode one
// legitimately huge payload, has its scratch buffer capped rather than
// pinned at that size once returned via PutDecoder. String decoding
// routes through d.buf (see stringWithLen / readN); a large []byte
// destination would not exercise it, since decodeBytesPtr reads directly
// into the caller's slice instead.
func TestPooledDecoderDoesNotRetainHugePayloadCapacity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large allocation test in -short mode")
	}

	const size = maxPooledBufSize * 4
	big := make([]byte, size)
	for i := range big {
		big[i] = 'a' + byte(i%26)
	}
	src := string(big)
	data, err := Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	dec := GetDecoder()
	dec.UsePreallocateValues(true)
	dec.Reset(bytes.NewReader(data))

	var out string
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != size {
		t.Fatalf("decoded len=%d, want %d", len(out), size)
	}
	if cap(dec.buf) < size {
		t.Fatalf("sanity check failed: dec.buf did not grow to the payload size, cap=%d", cap(dec.buf))
	}

	PutDecoder(dec)

	if cap(dec.buf) > maxPooledBufSize {
		t.Fatalf("pooled decoder retained oversized buffer: cap=%d limit=%d", cap(dec.buf), maxPooledBufSize)
	}
}

func TestPutDecoderKeepsSmallBuffers(t *testing.T) {
	dec := NewDecoder(nil)
	dec.buf = make([]byte, 0, 128)

	PutDecoder(dec)

	if cap(dec.buf) != 128 {
		t.Fatalf("expected small buf to survive Put unchanged, cap=%d", cap(dec.buf))
	}
}

func TestPutEncoderDropsOversizedBuffers(t *testing.T) {
	enc := NewEncoder(nil)
	enc.buf = make([]byte, 9, maxPooledBufSize+1)
	enc.appendBuf.b = make([]byte, 0, maxPooledBufSize+1)

	PutEncoder(enc)

	if cap(enc.buf) != 9 {
		t.Fatalf("expected oversized buf to be reset to the initial capacity, cap=%d", cap(enc.buf))
	}
	if enc.appendBuf.b != nil {
		t.Fatalf("expected oversized appendBuf.b to be dropped, cap=%d", cap(enc.appendBuf.b))
	}

	// The reset buffer must remain usable: write1/write2/write4/write8
	// slice it unconditionally and expect cap >= 9.
	var buf bytes.Buffer
	enc.Reset(&buf)
	if err := enc.EncodeUint64(42); err != nil {
		t.Fatalf("encode after reset buf: %v", err)
	}
}

// TestPooledEncoderDoesNotRetainHugeAppendCapacity mirrors
// TestPooledDecoderDoesNotRetainHugePayloadCapacity for the encoder side:
// Encoder.Append assigns the caller's dst directly into enc.appendBuf.b,
// so a single huge AppendMarshal call must not leave that capacity
// pinned in the shared package-level pool.
func TestPooledEncoderDoesNotRetainHugeAppendCapacity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large allocation test in -short mode")
	}

	const size = maxPooledBufSize * 4
	big := make([]byte, size)

	enc := GetEncoder()
	enc.Reset(nil)
	dst, err := enc.Append(nil, string(big))
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if len(dst) < size {
		t.Fatalf("sanity check failed: encoded output shorter than payload, len=%d", len(dst))
	}
	if cap(enc.appendBuf.b) < size {
		t.Fatalf("sanity check failed: appendBuf.b did not grow to the payload size, cap=%d", cap(enc.appendBuf.b))
	}

	PutEncoder(enc)

	if cap(enc.appendBuf.b) > maxPooledBufSize {
		t.Fatalf("pooled encoder retained oversized appendBuf.b: cap=%d limit=%d", cap(enc.appendBuf.b), maxPooledBufSize)
	}
}

func TestPutEncoderKeepsSmallBuffers(t *testing.T) {
	enc := NewEncoder(nil)
	enc.appendBuf.b = make([]byte, 0, 128)

	PutEncoder(enc)

	if cap(enc.appendBuf.b) != 128 {
		t.Fatalf("expected small appendBuf.b to survive Put unchanged, cap=%d", cap(enc.appendBuf.b))
	}
}

// TestNewValueAllocatesDirectly is a smoke test for the simplified
// preallocator: newValue must return a settable, addressable zero value
// regardless of the UsePreallocateValues flag.
func TestNewValueAllocatesDirectly(t *testing.T) {
	dec := NewDecoder(nil)

	for _, prealloc := range []bool{false, true} {
		dec.UsePreallocateValues(prealloc)
		v := dec.newValue(stringType)
		if v.Kind().String() != "ptr" || v.IsNil() {
			t.Fatalf("prealloc=%v: newValue returned invalid value: %#v", prealloc, v)
		}
		v.Elem().SetString("x")
		if v.Elem().String() != "x" {
			t.Fatalf("prealloc=%v: value not settable", prealloc)
		}
	}
}
