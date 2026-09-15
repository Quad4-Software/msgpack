package msgpack_test

// Round-trip oracle tests: encode a value, decode into a fresh value of
// the same Go type, and require deep equality. These cover nested
// structs, maps, slices, field tags, and omitempty end to end, plus a
// realistic mixed payload that exercises the whole stack in one shot.

import (
	"testing"
	"time"

	"github.com/Quad4-Software/msgpack/v5/pkg/msgpack"
)

type oracleItem struct {
	Key   string `msgpack:"key"`
	Value int    `msgpack:"value"`
}

type oracleNested struct {
	ID    int64             `msgpack:"id"`
	Name  string            `msgpack:"name"`
	Skip  string            `msgpack:"skip,omitempty"`
	Tags  map[string]string `msgpack:"tags"`
	Items []oracleItem      `msgpack:"items"`
	Inner *oracleItem       `msgpack:"inner"`
}

func TestRoundTripOracleNestedStruct(t *testing.T) {
	in := oracleNested{
		ID:   42,
		Name: "root",
		Tags: map[string]string{"env": "test", "region": "us"},
		Items: []oracleItem{
			{Key: "a", Value: 1},
			{Key: "b", Value: -2},
		},
		Inner: &oracleItem{Key: "leaf", Value: 7},
	}

	data, err := msgpack.Marshal(in)
	mustOK(t, err)

	var out oracleNested
	mustOK(t, msgpack.Unmarshal(data, &out))
	mustDeepEqual(t, out, in)
}

// TestRoundTripOracleOmitEmpty asserts that a zero omitempty field is
// absent from the wire and that the decoded struct matches a manually
// constructed expectation rather than just itself.
func TestRoundTripOracleOmitEmpty(t *testing.T) {
	in := oracleNested{ID: 1, Name: "n"} // Skip stays empty.
	data, err := msgpack.Marshal(in)
	mustOK(t, err)

	// The map on the wire must not carry the "skip" key at all.
	var asMap map[string]any
	mustOK(t, msgpack.Unmarshal(data, &asMap))
	if _, ok := asMap["skip"]; ok {
		t.Fatalf("omitempty field present on wire: %#v", asMap)
	}
	// id, name, tags, items, inner: five keys with skip omitted.
	mustEqual(t, len(asMap), 5)

	var out oracleNested
	mustOK(t, msgpack.Unmarshal(data, &out))
	mustEqual(t, out.Skip, "")
	mustEqual(t, out.ID, in.ID)
	mustEqual(t, out.Name, in.Name)
}

// TestRoundTripOracleEmptyContainers pins the decode of empty maps and
// slices: an empty map decodes to an empty non-nil map, and an empty
// slice decodes to an empty non-nil slice, matching encoder output.
func TestRoundTripOracleEmptyContainers(t *testing.T) {
	t.Run("empty_map", func(t *testing.T) {
		in := map[string]int{}
		data, err := msgpack.Marshal(in)
		mustOK(t, err)
		mustBytesEqual(t, data, []byte{0x80})

		var out map[string]int
		mustOK(t, msgpack.Unmarshal(data, &out))
		mustDeepEqual(t, out, in)
	})

	t.Run("empty_slice", func(t *testing.T) {
		in := []int{}
		data, err := msgpack.Marshal(in)
		mustOK(t, err)
		mustBytesEqual(t, data, []byte{0x90})

		var out []int
		mustOK(t, msgpack.Unmarshal(data, &out))
		mustDeepEqual(t, out, in)
	})

	t.Run("nested_empty", func(t *testing.T) {
		in := map[string]any{
			"m": map[string]any{},
			"s": []any{},
		}
		data, err := msgpack.Marshal(in)
		mustOK(t, err)
		var out map[string]any
		mustOK(t, msgpack.Unmarshal(data, &out))
		mustDeepEqual(t, out, in)
	})
}

// mixedPayload models a realistic end-to-end message: scalars of every
// width family, byte payloads, nested containers, and a timestamp.
type mixedPayload struct {
	RequestID string            `msgpack:"request_id"`
	Attempt   int               `msgpack:"attempt"`
	OK        bool              `msgpack:"ok"`
	Score     float64           `msgpack:"score"`
	Blob      []byte            `msgpack:"blob"`
	Labels    map[string]string `msgpack:"labels"`
	Counts    []int64           `msgpack:"counts"`
	Deadline  time.Time         `msgpack:"deadline"`
	Timeout   time.Duration     `msgpack:"timeout"`
	Detail    oracleItem        `msgpack:"detail"`
	Note      string            `msgpack:"note,omitempty"`
}

// TestMixedPayloadSmoke is the end-to-end smoke test: a realistic
// heterogeneous struct must survive Marshal and Unmarshal byte for
// byte in value terms, including the timestamp ext and the duration
// int64 path.
func TestMixedPayloadSmoke(t *testing.T) {
	in := mixedPayload{
		RequestID: "req-7f3a",
		Attempt:   3,
		OK:        true,
		Score:     0.9375,
		Blob:      []byte{0x00, 0x01, 0xfe, 0xff},
		Labels:    map[string]string{"service": "relay", "shard": "12"},
		Counts:    []int64{0, -1, 1 << 40},
		Deadline:  time.Unix(1700000000, 123456789).UTC(),
		Timeout:   250 * time.Millisecond,
		Detail:    oracleItem{Key: "origin", Value: 99},
		Note:      "present",
	}

	data, err := msgpack.Marshal(in)
	mustOK(t, err)

	var out mixedPayload
	mustOK(t, msgpack.Unmarshal(data, &out))
	// The decoded time is normalized to UTC, so compare instants rather
	// than relying on DeepEqual over the location pointer.
	mustTrue(t, out.Deadline.Equal(in.Deadline), "deadline mismatch")
	in.Deadline, out.Deadline = time.Time{}, time.Time{}
	mustDeepEqual(t, out, in)

	// Determinism: two encoders with sorted map keys must produce the
	// identical byte stream for the same value.
	var b1, b2 []byte
	enc1 := msgpack.NewEncoder(nil)
	enc1.SetSortMapKeys(true)
	enc2 := msgpack.NewEncoder(nil)
	enc2.SetSortMapKeys(true)
	b1, err = enc1.Append(b1, in)
	mustOK(t, err)
	b2, err = enc2.Append(b2, in)
	mustOK(t, err)
	mustBytesEqual(t, b1, b2)
}
