package msgpack_test

import (
	"bytes"
	"math"
	"testing"

	"git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack"
	"git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack/msgpcode"
)

// TestInvariantUnmarshalNilOrEmpty confirms that the high-level Unmarshal
// surface returns a non-nil error and does not panic for empty or nil input.
func TestInvariantUnmarshalNilOrEmpty(t *testing.T) {
	cases := [][]byte{nil, {}}
	var dst interface{}
	for _, data := range cases {
		err := msgpack.Unmarshal(data, &dst)
		if err == nil {
			t.Fatalf("expected error for input %v", data)
		}
	}
}

// TestInvariantMarshalNil verifies that Marshal(nil) returns a single
// nil-code byte. Any change here is a wire-format break and must be loud.
func TestInvariantMarshalNil(t *testing.T) {
	data, err := msgpack.Marshal(nil)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(data, []byte{msgpcode.Nil}) {
		t.Fatalf("nil encoding changed: got % x", data)
	}
}

// TestInvariantScalarRoundtrip checks bit-exact round-tripping for a small
// set of edge-case scalars that historically broke encoder rewrites.
func TestInvariantScalarRoundtrip(t *testing.T) {
	t.Run("int64Min", func(t *testing.T) {
		var out int64
		roundtripScalar(t, int64(math.MinInt64), &out)
		if out != math.MinInt64 {
			t.Fatalf("got %d", out)
		}
	})
	t.Run("uint64Max", func(t *testing.T) {
		var out uint64
		roundtripScalar(t, uint64(math.MaxUint64), &out)
		if out != math.MaxUint64 {
			t.Fatalf("got %d", out)
		}
	})
	t.Run("float64NaN", func(t *testing.T) {
		var out float64
		roundtripScalar(t, math.NaN(), &out)
		if !math.IsNaN(out) {
			t.Fatalf("expected NaN, got %v", out)
		}
	})
	t.Run("float64Inf", func(t *testing.T) {
		var out float64
		roundtripScalar(t, math.Inf(-1), &out)
		if !math.IsInf(out, -1) {
			t.Fatalf("expected -Inf, got %v", out)
		}
	})
}

func roundtripScalar(t *testing.T, in, out interface{}) {
	t.Helper()
	data, err := msgpack.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := msgpack.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

// TestInvariantPoolDoesNotAlias guards against a regression where the
// pooled bytes.Reader leaks the previous caller's data into a subsequent
// Unmarshal call. The second Unmarshal of an empty slice must succeed
// (with the expected EOF/empty-input error) rather than decode stale bytes.
func TestInvariantPoolDoesNotAlias(t *testing.T) {
	first, err := msgpack.Marshal("first call")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var s string
	if err := msgpack.Unmarshal(first, &s); err != nil {
		t.Fatalf("first unmarshal: %v", err)
	}
	if s != "first call" {
		t.Fatalf("first decode: got %q", s)
	}

	var s2 string
	if err := msgpack.Unmarshal(nil, &s2); err == nil {
		t.Fatalf("second unmarshal: expected error on nil input, decoded %q", s2)
	}
	if s2 != "" {
		t.Fatalf("second decode leaked data: got %q", s2)
	}
}
