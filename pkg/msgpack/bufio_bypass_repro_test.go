package msgpack_test

import (
	"bufio"
	"bytes"
	"io"
	"runtime"
	"testing"

	"quad4/msgpack/v5/pkg/msgpack"
)

// limitedReader is an io.Reader without Len or ByteScanner, forcing the
// decoder through newBufferedReader without remaining-input accounting.
type limitedReader struct {
	r io.Reader
}

func (l limitedReader) Read(p []byte) (int, error) { return l.r.Read(p) }

func TestBufioReaderRejectsForgedArray32(t *testing.T) {
	payload := []byte{0xdd, 0xff, 0xff, 0xff, 0xff}

	decBytes := msgpack.NewDecoder(bytes.NewReader(payload))
	nBytes, errBytes := decBytes.DecodeArrayLen()
	assertOversizedContainerRejected(t, "bytes.Reader array length", nBytes, errBytes)

	// Externally constructed *bufio.Reader has no Len. Soft alloc ceiling
	// must still reject MaxUint32.
	decBufio := msgpack.NewDecoder(bufio.NewReader(bytes.NewReader(payload)))
	n, errBufio := decBufio.DecodeArrayLen()
	assertOversizedContainerRejected(t, "bufio.Reader array length", n, errBufio)

	decPlain := msgpack.NewDecoder(limitedReader{r: bytes.NewReader(payload)})
	nPlain, errPlain := decPlain.DecodeArrayLen()
	assertOversizedContainerRejected(t, "plain reader array length", nPlain, errPlain)
}

func TestBufioReaderDecodeDoesNotAllocateGigabytes(t *testing.T) {
	payload := []byte{0xdd, 0xff, 0xff, 0xff, 0xff}
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	var out any
	err := msgpack.NewDecoder(bufio.NewReader(bytes.NewReader(payload))).Decode(&out)
	runtime.ReadMemStats(&m2)
	if err == nil {
		t.Fatal("expected forged array32 via bufio to fail")
	}
	if !isFailFastContainerError(err) {
		t.Fatalf("expected fail-fast container error, got %v", err)
	}
	alloc := m2.TotalAlloc - m1.TotalAlloc
	const maxAlloc = 32 << 20
	if alloc > maxAlloc {
		t.Fatalf("bufio forged array allocated %d bytes, want <= %d", alloc, maxAlloc)
	}
}

func TestRejectForgedBin32Length(t *testing.T) {
	dec := msgpack.NewDecoder(bytes.NewReader([]byte{0xc6, 0xff, 0xff, 0xff, 0xff}))
	n, err := dec.DecodeBytesLen()
	// On 64-bit this is usually remaining-input. On 32-bit, MaxUint32
	// overflows int first via uint32ToInt.
	assertOversizedBytesRejected(t, "forged bin32", n, err)
}

func TestSoftCeilingRejectsForgedMap32WithoutLen(t *testing.T) {
	payload := []byte{0xdf, 0xff, 0xff, 0xff, 0xff}
	dec := msgpack.NewDecoder(limitedReader{bytes.NewReader(payload)})
	n, err := dec.DecodeMapLen()
	assertOversizedContainerRejected(t, "wrapped map length", n, err)
}
