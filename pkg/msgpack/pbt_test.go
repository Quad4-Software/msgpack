package msgpack_test

import (
	"bytes"
	"reflect"
	"testing"

	"quad4/msgpack/v5/pkg/msgpack"
	"quad4/pbt/pkg/pbt"
)

func intSliceToBytes(xs []int) []byte {
	b := make([]byte, len(xs))
	for i, v := range xs {
		b[i] = byte(v)
	}
	return b
}

func TestPBTRoundtripBytes(t *testing.T) {
	gen := pbt.Map("[]byte",
		pbt.SliceOf(pbt.IntRange(0, 255), 0, 16384),
		intSliceToBytes,
	)
	prop := pbt.ForAll(
		"Marshal then Unmarshal []byte roundtrip",
		gen,
		func(data []byte) bool {
			b, err := msgpack.Marshal(data)
			if err != nil {
				return false
			}
			var out []byte
			if err := msgpack.Unmarshal(b, &out); err != nil {
				return false
			}
			return bytes.Equal(out, data)
		},
		pbt.WithShrinker[[]byte](pbt.SliceShrinker[byte]()),
	)
	pbt.Check(t, prop, pbt.WithRuns(200), pbt.WithSeed(42))
}

func TestPBTRoundtripString(t *testing.T) {
	gen := pbt.Map("string",
		pbt.SliceOf(pbt.IntRange(0, 255), 0, 4096),
		func(xs []int) string {
			b := make([]byte, len(xs))
			for i, v := range xs {
				b[i] = byte(v)
			}
			return string(b)
		},
	)
	prop := pbt.ForAll(
		"Marshal then Unmarshal string roundtrip",
		gen,
		func(s string) bool {
			b, err := msgpack.Marshal(s)
			if err != nil {
				return false
			}
			var out string
			if err := msgpack.Unmarshal(b, &out); err != nil {
				return false
			}
			return out == s
		},
		pbt.WithShrinker[string](pbt.StringShrinker()),
	)
	pbt.Check(t, prop, pbt.WithRuns(200), pbt.WithSeed(43))
}

func pairsToMap(pairs []pbt.Tuple2Value[string, int]) map[string]int {
	m := make(map[string]int, len(pairs))
	for _, p := range pairs {
		m[p.First] = p.Second
	}
	return m
}

func TestPBTRoundtripMapStringInt(t *testing.T) {
	pairGen := pbt.Tuple2("kv",
		pbt.StringASCII(1, 16),
		pbt.IntRange(-1<<20, 1<<20),
	)
	gen := pbt.Map("map[string]int",
		pbt.SliceOf(pairGen, 0, 64),
		pairsToMap,
	)
	prop := pbt.ForAll(
		"Marshal then Unmarshal map[string]int roundtrip",
		gen,
		func(m map[string]int) bool {
			b, err := msgpack.Marshal(m)
			if err != nil {
				return false
			}
			var out map[string]int
			if err := msgpack.Unmarshal(b, &out); err != nil {
				return false
			}
			return reflect.DeepEqual(m, out)
		},
	)
	pbt.Check(t, prop, pbt.WithRuns(150), pbt.WithSeed(44))
}

func TestPBTRoundtripIntSlice(t *testing.T) {
	gen := pbt.SliceOf(pbt.IntRange(-1<<30, 1<<30), 0, 256)
	prop := pbt.ForAll(
		"Marshal then Unmarshal []int roundtrip",
		gen,
		func(xs []int) bool {
			b, err := msgpack.Marshal(xs)
			if err != nil {
				return false
			}
			var out []int
			if err := msgpack.Unmarshal(b, &out); err != nil {
				return false
			}
			if len(xs) == 0 && len(out) == 0 {
				return true
			}
			return reflect.DeepEqual(out, xs)
		},
		pbt.WithShrinker[[]int](pbt.SliceShrinker[int]()),
	)
	pbt.Check(t, prop, pbt.WithRuns(150), pbt.WithSeed(45))
}

func TestPBTRoundtripStringSlice(t *testing.T) {
	gen := pbt.SliceOf(pbt.StringASCII(0, 16), 0, 64)
	prop := pbt.ForAll(
		"Marshal then Unmarshal []string roundtrip",
		gen,
		func(xs []string) bool {
			b, err := msgpack.Marshal(xs)
			if err != nil {
				return false
			}
			var out []string
			if err := msgpack.Unmarshal(b, &out); err != nil {
				return false
			}
			if len(xs) == 0 && len(out) == 0 {
				return true
			}
			return reflect.DeepEqual(out, xs)
		},
		pbt.WithShrinker[[]string](pbt.SliceShrinker[string]()),
	)
	pbt.Check(t, prop, pbt.WithRuns(150), pbt.WithSeed(46))
}

func pairsToStrMap(pairs []pbt.Tuple2Value[string, string]) map[string]string {
	m := make(map[string]string, len(pairs))
	for _, p := range pairs {
		m[p.First] = p.Second
	}
	return m
}

func TestPBTRoundtripMapStringString(t *testing.T) {
	pairGen := pbt.Tuple2("kv",
		pbt.StringASCII(1, 16),
		pbt.StringASCII(0, 32),
	)
	gen := pbt.Map("map[string]string",
		pbt.SliceOf(pairGen, 0, 64),
		pairsToStrMap,
	)
	prop := pbt.ForAll(
		"Marshal then Unmarshal map[string]string roundtrip",
		gen,
		func(m map[string]string) bool {
			b, err := msgpack.Marshal(m)
			if err != nil {
				return false
			}
			var out map[string]string
			if err := msgpack.Unmarshal(b, &out); err != nil {
				return false
			}
			return reflect.DeepEqual(m, out)
		},
	)
	pbt.Check(t, prop, pbt.WithRuns(150), pbt.WithSeed(47))
}
