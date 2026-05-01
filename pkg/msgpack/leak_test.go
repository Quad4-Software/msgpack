package msgpack_test

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack"
)

// TestNoGoroutineLeakOnDistinctTypes asserts that decoding into a large
// number of distinct Go types does not leak goroutines. The previous
// preallocator spawned one perpetual goroutine per distinct type, so a
// long-running process that ever decoded into 10k different types
// retained 10k goroutines forever.
func TestNoGoroutineLeakOnDistinctTypes(t *testing.T) {
	runtime.GC()
	before := runtime.NumGoroutine()

	const types = 256
	for i := range types {
		typ := reflect.StructOf([]reflect.StructField{{
			Name: fmt.Sprintf("F%d", i),
			Type: reflect.TypeFor[int64](),
		}})
		ptr := reflect.New(typ).Interface()
		data, err := msgpack.Marshal(map[string]int64{fmt.Sprintf("F%d", i): int64(i)})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := msgpack.Unmarshal(data, ptr); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
	}

	runtime.GC()
	after := runtime.NumGoroutine()

	// Tolerate a small constant overhead from the test harness itself
	// (race detector worker, GC mark workers, etc.). The previous
	// implementation would have grown by ~256 goroutines.
	if delta := after - before; delta > 8 {
		t.Fatalf("goroutine leak: before=%d after=%d delta=%d", before, after, delta)
	}
}

// TestNoGoroutineLeakOnRepeatedDecode confirms that hammering Unmarshal
// for the same type many times also does not leak.
func TestNoGoroutineLeakOnRepeatedDecode(t *testing.T) {
	type payload struct {
		A int64
		B string
		C []int
	}

	src := payload{A: 1, B: "x", C: []int{1, 2, 3}}
	data, err := msgpack.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	runtime.GC()
	before := runtime.NumGoroutine()

	for range 5000 {
		var dst payload
		if err := msgpack.Unmarshal(data, &dst); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
	}

	runtime.GC()
	after := runtime.NumGoroutine()

	if delta := after - before; delta > 4 {
		t.Fatalf("goroutine leak: before=%d after=%d delta=%d", before, after, delta)
	}
}

// TestConcurrentDistinctTypesPreallocate hammers cachedValue from many
// goroutines with many distinct types simultaneously. The previous
// channel-based implementation serialized through a single per-type
// goroutine; the sync.Pool replacement must remain race-free under
// contention while still avoiding goroutine accumulation.
func TestConcurrentDistinctTypesPreallocate(t *testing.T) {
	type t1 struct{ A int64 }
	type t2 struct{ B string }
	type t3 struct{ C []byte }
	type t4 struct{ D map[string]int64 }

	d1, _ := msgpack.Marshal(t1{A: 1})
	d2, _ := msgpack.Marshal(t2{B: "x"})
	d3, _ := msgpack.Marshal(t3{C: []byte{1, 2, 3}})
	d4, _ := msgpack.Marshal(t4{D: map[string]int64{"k": 1}})

	runtime.GC()
	before := runtime.NumGoroutine()

	const goroutines = 32
	const iters = 200
	done := make(chan struct{}, goroutines)
	for range goroutines {
		go func() {
			defer func() { done <- struct{}{} }()
			for range iters {
				var a t1
				var b t2
				var c t3
				var d t4
				if err := msgpack.Unmarshal(d1, &a); err != nil {
					t.Errorf("a: %v", err)
					return
				}
				if err := msgpack.Unmarshal(d2, &b); err != nil {
					t.Errorf("b: %v", err)
					return
				}
				if err := msgpack.Unmarshal(d3, &c); err != nil {
					t.Errorf("c: %v", err)
					return
				}
				if err := msgpack.Unmarshal(d4, &d); err != nil {
					t.Errorf("d: %v", err)
					return
				}
			}
		}()
	}
	for range goroutines {
		<-done
	}

	runtime.GC()
	after := runtime.NumGoroutine()
	if delta := after - before; delta > 8 {
		t.Fatalf("goroutine leak under contention: before=%d after=%d delta=%d", before, after, delta)
	}
}

// TestPoolDoesNotRetainCallerData verifies that the pooled bytes.Reader
// in Unmarshal does not keep a reference to the previous caller's data
// after the call returns. We Marshal a unique slice, Unmarshal it, then
// Unmarshal a fresh slice and confirm the second decode does not leak
// data from the first.
func TestPoolDoesNotRetainCallerData(t *testing.T) {
	a, err := msgpack.Marshal("first")
	if err != nil {
		t.Fatalf("marshal a: %v", err)
	}
	var s string
	if err := msgpack.Unmarshal(a, &s); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if s != "first" {
		t.Fatalf("decode a: got %q", s)
	}

	b, err := msgpack.Marshal("second")
	if err != nil {
		t.Fatalf("marshal b: %v", err)
	}
	var s2 string
	if err := msgpack.Unmarshal(b, &s2); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}
	if s2 != "second" {
		t.Fatalf("decode b: got %q", s2)
	}
}
