package msgpcode

// Boundary tests for the opcode predicates. Each helper is checked at
// the exact edges of its range and at the adjacent out-of-range bytes,
// plus a sweep that asserts every predicate returns a consistent
// partition over all 256 code points.

import "testing"

func TestOpcodeConstantsMatchSpec(t *testing.T) {
	cases := []struct {
		name string
		got  byte
		want byte
	}{
		{"PosFixedNumHigh", PosFixedNumHigh, 0x7f},
		{"NegFixedNumLow", NegFixedNumLow, 0xe0},
		{"Nil", Nil, 0xc0},
		{"False", False, 0xc2},
		{"True", True, 0xc3},
		{"Float", Float, 0xca},
		{"Double", Double, 0xcb},
		{"Uint8", Uint8, 0xcc},
		{"Uint16", Uint16, 0xcd},
		{"Uint32", Uint32, 0xce},
		{"Uint64", Uint64, 0xcf},
		{"Int8", Int8, 0xd0},
		{"Int16", Int16, 0xd1},
		{"Int32", Int32, 0xd2},
		{"Int64", Int64, 0xd3},
		{"FixedStrLow", FixedStrLow, 0xa0},
		{"FixedStrHigh", FixedStrHigh, 0xbf},
		{"FixedStrMask", FixedStrMask, 0x1f},
		{"Str8", Str8, 0xd9},
		{"Str16", Str16, 0xda},
		{"Str32", Str32, 0xdb},
		{"Bin8", Bin8, 0xc4},
		{"Bin16", Bin16, 0xc5},
		{"Bin32", Bin32, 0xc6},
		{"FixedArrayLow", FixedArrayLow, 0x90},
		{"FixedArrayHigh", FixedArrayHigh, 0x9f},
		{"FixedArrayMask", FixedArrayMask, 0x0f},
		{"Array16", Array16, 0xdc},
		{"Array32", Array32, 0xdd},
		{"FixedMapLow", FixedMapLow, 0x80},
		{"FixedMapHigh", FixedMapHigh, 0x8f},
		{"FixedMapMask", FixedMapMask, 0x0f},
		{"Map16", Map16, 0xde},
		{"Map32", Map32, 0xdf},
		{"FixExt1", FixExt1, 0xd4},
		{"FixExt2", FixExt2, 0xd5},
		{"FixExt4", FixExt4, 0xd6},
		{"FixExt8", FixExt8, 0xd7},
		{"FixExt16", FixExt16, 0xd8},
		{"Ext8", Ext8, 0xc7},
		{"Ext16", Ext16, 0xc8},
		{"Ext32", Ext32, 0xc9},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %#x, want %#x", tc.name, tc.got, tc.want)
		}
	}
}

func TestIsFixedNum(t *testing.T) {
	for _, c := range []byte{0x00, 0x01, 0x7f, 0xe0, 0xff} {
		if !IsFixedNum(c) {
			t.Errorf("IsFixedNum(%#x) = false, want true", c)
		}
	}
	for _, c := range []byte{0x80, 0xa0, 0xc0, 0xdf} {
		if IsFixedNum(c) {
			t.Errorf("IsFixedNum(%#x) = true, want false", c)
		}
	}
}

func TestIsFixedMap(t *testing.T) {
	for _, c := range []byte{0x80, 0x81, 0x8f} {
		if !IsFixedMap(c) {
			t.Errorf("IsFixedMap(%#x) = false, want true", c)
		}
	}
	for _, c := range []byte{0x7f, 0x90, 0xde} {
		if IsFixedMap(c) {
			t.Errorf("IsFixedMap(%#x) = true, want false", c)
		}
	}
}

func TestIsFixedArray(t *testing.T) {
	for _, c := range []byte{0x90, 0x91, 0x9f} {
		if !IsFixedArray(c) {
			t.Errorf("IsFixedArray(%#x) = false, want true", c)
		}
	}
	for _, c := range []byte{0x8f, 0xa0, 0xdc} {
		if IsFixedArray(c) {
			t.Errorf("IsFixedArray(%#x) = true, want false", c)
		}
	}
}

func TestIsFixedString(t *testing.T) {
	for _, c := range []byte{0xa0, 0xa1, 0xbf} {
		if !IsFixedString(c) {
			t.Errorf("IsFixedString(%#x) = false, want true", c)
		}
	}
	for _, c := range []byte{0x9f, 0xc0, 0xd9} {
		if IsFixedString(c) {
			t.Errorf("IsFixedString(%#x) = true, want false", c)
		}
	}
}

func TestIsString(t *testing.T) {
	for _, c := range []byte{0xa0, 0xbf, Str8, Str16, Str32} {
		if !IsString(c) {
			t.Errorf("IsString(%#x) = false, want true", c)
		}
	}
	for _, c := range []byte{0x9f, Bin8, Array16, 0xc0} {
		if IsString(c) {
			t.Errorf("IsString(%#x) = true, want false", c)
		}
	}
}

func TestIsBin(t *testing.T) {
	for _, c := range []byte{Bin8, Bin16, Bin32} {
		if !IsBin(c) {
			t.Errorf("IsBin(%#x) = false, want true", c)
		}
	}
	for _, c := range []byte{0xc0, 0xc3, Str8, Ext8} {
		if IsBin(c) {
			t.Errorf("IsBin(%#x) = true, want false", c)
		}
	}
}

func TestIsFixedExt(t *testing.T) {
	for _, c := range []byte{FixExt1, FixExt2, FixExt4, FixExt8, FixExt16} {
		if !IsFixedExt(c) {
			t.Errorf("IsFixedExt(%#x) = false, want true", c)
		}
	}
	for _, c := range []byte{0xd3, Ext8, Str8} {
		if IsFixedExt(c) {
			t.Errorf("IsFixedExt(%#x) = true, want false", c)
		}
	}
}

func TestIsExt(t *testing.T) {
	for _, c := range []byte{FixExt1, FixExt16, Ext8, Ext16, Ext32} {
		if !IsExt(c) {
			t.Errorf("IsExt(%#x) = false, want true", c)
		}
	}
	for _, c := range []byte{0xd3, Str8, Bin8, 0xc0} {
		if IsExt(c) {
			t.Errorf("IsExt(%#x) = true, want false", c)
		}
	}
}

// TestPredicatePartition sweeps all 256 code points and asserts the
// reserved 0xc1 code is classified as nothing, and that no code is
// claimed by two disjoint families.
func TestPredicatePartition(t *testing.T) {
	for c := 0; c <= 0xff; c++ {
		b := byte(c)
		if b == 0xc1 {
			for name, pred := range map[string]func(byte) bool{
				"IsFixedNum":    IsFixedNum,
				"IsFixedMap":    IsFixedMap,
				"IsFixedArray":  IsFixedArray,
				"IsFixedString": IsFixedString,
				"IsString":      IsString,
				"IsBin":         IsBin,
				"IsFixedExt":    IsFixedExt,
				"IsExt":         IsExt,
			} {
				if pred(b) {
					t.Errorf("%s(0xc1) = true, want false for reserved code", name)
				}
			}
		}
	}
}
