package msgpack_test

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack"
)

// TestDecodeFloatIntoInt64_JSONRoundtrip pins the JSON -> map -> msgpack
// -> struct round-trip: encoding/json decodes JSON numbers into float64
// regardless of declared field type, so the wire payload uses the Double
// code (cb) even though the destination is int64. The decoder must
// accept this and cleanly convert.
func TestDecodeFloatIntoInt64_JSONRoundtrip(t *testing.T) {
	type Test struct {
		ID int64 `json:"id"`
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(`{"id":10}`), &obj); err != nil {
		t.Fatalf("json: %v", err)
	}

	data, err := msgpack.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out Test
	dec := msgpack.NewDecoder(bytes.NewReader(data))
	dec.SetCustomStructTag("json")
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != 10 {
		t.Fatalf("ID = %d, want 10", out.ID)
	}
}

// TestDecodeFloatIntoInt64_FractionalRejected confirms the loose float
// path only accepts values that are exactly representable as the target
// integer type. Anything with a fractional component or out of range is
// rejected so silent truncation of meaningful data is impossible.
func TestDecodeFloatIntoInt64_FractionalRejected(t *testing.T) {
	cases := []float64{
		1.5,
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		math.MaxFloat64,
	}
	for _, f := range cases {
		data, err := msgpack.Marshal(f)
		if err != nil {
			t.Fatalf("marshal %v: %v", f, err)
		}
		var n int64
		if err := msgpack.Unmarshal(data, &n); err == nil {
			t.Fatalf("expected error decoding %v into int64, got %d", f, n)
		}
	}
}

// TestDecodeFloatIntoUint64 checks the uint64 path mirrors the int64
// path: integer-valued, in-range floats round-trip cleanly while
// negative or fractional values are rejected.
func TestDecodeFloatIntoUint64(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		data, err := msgpack.Marshal(float64(42))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var n uint64
		if err := msgpack.Unmarshal(data, &n); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if n != 42 {
			t.Fatalf("got %d", n)
		}
	})
	t.Run("negative-rejected", func(t *testing.T) {
		data, err := msgpack.Marshal(float64(-1))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var n uint64
		if err := msgpack.Unmarshal(data, &n); err == nil {
			t.Fatalf("expected error, got %d", n)
		}
	})
	t.Run("fractional-rejected", func(t *testing.T) {
		data, err := msgpack.Marshal(float64(2.5))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var n uint64
		if err := msgpack.Unmarshal(data, &n); err == nil {
			t.Fatalf("expected error, got %d", n)
		}
	})
}

// TestDecodeFloat32IntoInt32 covers the Float (single-precision) wire
// code on the integer path. encoding/json never emits Float, but
// hand-crafted producers do.
func TestDecodeFloat32IntoInt32(t *testing.T) {
	data, err := msgpack.Marshal(float32(7))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var n int32
	if err := msgpack.Unmarshal(data, &n); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if n != 7 {
		t.Fatalf("got %d", n)
	}
}
