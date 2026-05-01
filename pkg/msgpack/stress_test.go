package msgpack_test

import (
	"bytes"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack"
)

// TestStressConcurrentMarshalUnmarshal hammers the package-level encoder
// and decoder pools from many goroutines. Combined with -race this catches
// any state that escapes the per-call Get / Put lifecycle.
func TestStressConcurrentMarshalUnmarshal(t *testing.T) {
	const (
		workers = 32
		perGoro = 200
	)

	type payload struct {
		Name   string
		Values []int
		Tags   map[string]string
	}

	src := payload{
		Name:   "stress",
		Values: []int{1, 2, 3, 4, 5, 6, 7, 8},
		Tags:   map[string]string{"k": "v", "a": "b"},
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for range perGoro {
				data, err := msgpack.Marshal(src)
				if err != nil {
					t.Errorf("marshal: %v", err)
					return
				}
				var dst payload
				if err := msgpack.Unmarshal(data, &dst); err != nil {
					t.Errorf("unmarshal: %v", err)
					return
				}
				if !reflect.DeepEqual(dst, src) {
					t.Errorf("mismatch: %#v", dst)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestStressLargeByteSlice round-trips a multi-megabyte byte slice to make
// sure the bin32 path and length prefixes survive larger-than-default
// allocation thresholds.
func TestStressLargeByteSlice(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large allocation test in -short mode")
	}
	const size = 1 << 21
	src := make([]byte, size)
	for i := range src {
		src[i] = byte(i)
	}
	data, err := msgpack.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var dst []byte
	if err := msgpack.Unmarshal(data, &dst); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !bytes.Equal(dst, src) {
		t.Fatalf("byte slice mismatch (len=%d)", len(dst))
	}
}

// TestStressLargeString round-trips a multi-megabyte string through the
// str32 path.
func TestStressLargeString(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large allocation test in -short mode")
	}
	const size = 1 << 21
	b := make([]byte, size)
	for i := range b {
		b[i] = byte('a' + (i % 26))
	}
	src := string(b)
	data, err := msgpack.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var dst string
	if err := msgpack.Unmarshal(data, &dst); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dst != src {
		t.Fatalf("string mismatch (len=%d)", len(dst))
	}
}

// TestStressDeepNestedMap exercises the recursive map decoder at depth.
// 16 levels is well below any stack or recursion limit but provides ample
// surface for use-after-pool or aliasing bugs to surface.
func TestStressDeepNestedMap(t *testing.T) {
	const depth = 16
	var leaf any = "leaf"
	for range depth {
		leaf = map[string]any{"k": leaf}
	}

	data, err := msgpack.Marshal(leaf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got any
	if err := msgpack.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cur := got
	for i := range depth {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("level %d: expected map, got %T", i, cur)
		}
		next, ok := m["k"]
		if !ok {
			t.Fatalf("level %d: missing key", i)
		}
		cur = next
	}
	if cur != "leaf" {
		t.Fatalf("leaf value: got %v", cur)
	}
}

// TestStressDeepNestedSlice exercises the recursive slice decoder.
func TestStressDeepNestedSlice(t *testing.T) {
	const depth = 16
	var leaf any = int64(42)
	for range depth {
		leaf = []any{leaf}
	}

	data, err := msgpack.Marshal(leaf)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got any
	if err := msgpack.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cur := got
	for i := range depth {
		s, ok := cur.([]any)
		if !ok {
			t.Fatalf("level %d: expected slice, got %T", i, cur)
		}
		if len(s) != 1 {
			t.Fatalf("level %d: len=%d", i, len(s))
		}
		cur = s[0]
	}
	if v, ok := cur.(int64); !ok || v != 42 {
		t.Fatalf("leaf value: got %T %v", cur, cur)
	}
}

// TestStressEncoderPoolReuse drives Get / Put repeatedly to confirm that
// recycled encoders do not retain state from previous calls.
func TestStressEncoderPoolReuse(t *testing.T) {
	const iters = 1000
	for i := range iters {
		enc := msgpack.GetEncoder()
		var buf bytes.Buffer
		enc.Reset(&buf)
		enc.SetSortMapKeys(i%2 == 0)
		enc.UseCompactInts(i%3 == 0)
		if err := enc.Encode(map[string]int{"a": i, "b": i + 1}); err != nil {
			t.Fatalf("encode: %v", err)
		}
		msgpack.PutEncoder(enc)

		var out map[string]int
		if err := msgpack.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out["a"] != i || out["b"] != i+1 {
			t.Fatalf("iter %d: got %v", i, out)
		}
	}
	runtime.GC()
}
