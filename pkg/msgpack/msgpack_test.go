package msgpack_test

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"git.quad4.io/Go-Libs/msgpack/v5/pkg/msgpack"
)

type nameStruct struct {
	Name string
}

type msgpackHarness struct {
	t   *testing.T
	buf *bytes.Buffer
	enc *msgpack.Encoder
	dec *msgpack.Decoder
}

func newMsgpackHarness(t *testing.T) *msgpackHarness {
	t.Helper()
	buf := &bytes.Buffer{}
	return &msgpackHarness{
		t:   t,
		buf: buf,
		enc: msgpack.NewEncoder(buf),
		dec: msgpack.NewDecoder(bufio.NewReader(buf)),
	}
}

func (h *msgpackHarness) mustEncode(v any) {
	h.t.Helper()
	mustOK(h.t, h.enc.Encode(v))
}

func (h *msgpackHarness) mustDecode(dst any) {
	h.t.Helper()
	mustOK(h.t, h.dec.Decode(dst))
}

func TestDecodeNil(t *testing.T) {
	h := newMsgpackHarness(t)
	mustErr(t, h.dec.Decode(nil))
}

func TestTime(t *testing.T) {
	t.Run("non_zero", func(t *testing.T) {
		h := newMsgpackHarness(t)
		in := time.Now()
		var out time.Time
		h.mustEncode(in)
		h.mustDecode(&out)
		mustTrue(t, out.Equal(in), fmt.Sprintf("got %v, want %v", out, in))
	})

	t.Run("zero", func(t *testing.T) {
		h := newMsgpackHarness(t)
		var zero, out time.Time
		h.mustEncode(zero)
		h.mustDecode(&out)
		mustTrue(t, out.Equal(zero), fmt.Sprintf("got %v, want zero", out))
		mustTrue(t, out.IsZero(), "expected zero time")
	})
}

func TestLargeBytes(t *testing.T) {
	h := newMsgpackHarness(t)
	const n = int(1e6)
	src := bytes.Repeat([]byte{'1'}, n)
	h.mustEncode(src)
	var dst []byte
	h.mustDecode(&dst)
	mustBytesEqual(t, dst, src)
}

func TestLargeString(t *testing.T) {
	h := newMsgpackHarness(t)
	const n = int(1e6)
	src := string(bytes.Repeat([]byte{'1'}, n))
	h.mustEncode(src)
	var dst string
	h.mustDecode(&dst)
	mustEqual(t, dst, src)
}

func TestSliceOfStructs(t *testing.T) {
	h := newMsgpackHarness(t)
	in := []*nameStruct{{"hello"}}
	var out []*nameStruct
	h.mustEncode(in)
	h.mustDecode(&out)
	mustDeepEqual(t, out, in)
}

func TestMapStringString(t *testing.T) {
	cases := []struct {
		name string
		m    map[string]string
		want []byte
	}{
		{
			name: "empty",
			m:    map[string]string{},
			want: []byte{0x80},
		},
		{
			name: "one_pair",
			m:    map[string]string{"hello": "world"},
			want: []byte{0x81, 0xa5, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0xa5, 0x77, 0x6f, 0x72, 0x6c, 0x64},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newMsgpackHarness(t)
			h.mustEncode(tc.m)
			mustBytesEqual(t, h.buf.Bytes(), tc.want)
			var m map[string]string
			h.mustDecode(&m)
			mustDeepEqual(t, m, tc.m)
		})
	}
}

func TestStructNil(t *testing.T) {
	h := newMsgpackHarness(t)
	var dst *nameStruct
	h.mustEncode(nameStruct{Name: "foo"})
	h.mustDecode(&dst)
	if dst == nil {
		t.Fatal("expected non-nil dst")
	}
	mustEqual(t, dst.Name, "foo")
}

func TestStructUnknownField(t *testing.T) {
	h := newMsgpackHarness(t)
	in := struct {
		Field1 string
		Field2 string
		Field3 string
	}{
		Field1: "value1",
		Field2: "value2",
		Field3: "value3",
	}
	h.mustEncode(in)
	out := struct {
		Field2 string
	}{}
	h.mustDecode(&out)
	mustEqual(t, out.Field2, "value2")
}

type coderStruct struct {
	name string
}

type wrapperStruct struct {
	coderStruct
}

var (
	_ msgpack.CustomEncoder = (*coderStruct)(nil)
	_ msgpack.CustomDecoder = (*coderStruct)(nil)
)

func (s *coderStruct) Name() string {
	return s.name
}

func (s *coderStruct) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.Encode(s.name)
}

func (s *coderStruct) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.Decode(&s.name)
}

func TestCustomCoderRoundTrip(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		h := newMsgpackHarness(t)
		in := &coderStruct{name: "hello"}
		var out coderStruct
		h.mustEncode(in)
		h.mustDecode(&out)
		mustEqual(t, out.Name(), "hello")
	})

	t.Run("nil_pointer", func(t *testing.T) {
		h := newMsgpackHarness(t)
		in := &coderStruct{name: "hello"}
		var out *coderStruct
		h.mustEncode(in)
		h.mustDecode(&out)
		mustEqual(t, out.Name(), "hello")
	})

	t.Run("decode_value", func(t *testing.T) {
		h := newMsgpackHarness(t)
		in := &coderStruct{name: "hello"}
		var out *coderStruct
		h.mustEncode(in)
		mustOK(t, h.dec.DecodeValue(reflect.ValueOf(&out)))
		mustEqual(t, out.Name(), "hello")
	})

	t.Run("pointer_to_value", func(t *testing.T) {
		h := newMsgpackHarness(t)
		in := &coderStruct{name: "hello"}
		var out coderStruct
		out2 := &out
		h.mustEncode(in)
		h.mustDecode(&out2)
		mustEqual(t, out.Name(), "hello")
	})

	t.Run("wrapped_embedded", func(t *testing.T) {
		h := newMsgpackHarness(t)
		in := &wrapperStruct{coderStruct: coderStruct{name: "hello"}}
		var out wrapperStruct
		h.mustEncode(in)
		h.mustDecode(&out)
		mustEqual(t, out.Name(), "hello")
	})
}

type struct2 struct {
	Name string
}

type struct1 struct {
	Name    string
	Struct2 struct2
}

func TestNestedStructs(t *testing.T) {
	h := newMsgpackHarness(t)
	in := &struct1{Name: "hello", Struct2: struct2{Name: "world"}}
	var out struct1
	h.mustEncode(in)
	h.mustDecode(&out)
	mustEqual(t, out.Name, in.Name)
	mustEqual(t, out.Struct2.Name, in.Struct2.Name)
}

type Struct4 struct {
	Name2 string
}

type Struct3 struct {
	Struct4
	Name1 string
}

func TestEmbedding(t *testing.T) {
	in := &Struct3{
		Name1: "hello",
		Struct4: Struct4{
			Name2: "world",
		},
	}
	var out Struct3

	b, err := msgpack.Marshal(in)
	mustOK(t, err)
	mustOK(t, msgpack.Unmarshal(b, &out))
	mustEqual(t, out.Name1, in.Name1)
	mustEqual(t, out.Name2, in.Name2)
}

func TestEmptyTimeMarshalWithInterface(t *testing.T) {
	a := time.Time{}
	b, err := msgpack.Marshal(a)
	mustOK(t, err)

	var out any
	mustOK(t, msgpack.Unmarshal(b, &out))
	name, _ := out.(time.Time).Zone()
	mustEqual(t, name, "UTC")

	var out2 time.Time
	mustOK(t, msgpack.Unmarshal(b, &out2))
	name, _ = out2.Zone()
	mustEqual(t, name, "UTC")
}

func TestSliceNil(t *testing.T) {
	h := newMsgpackHarness(t)
	in := [][]*int{nil}
	var out [][]*int
	h.mustEncode(in)
	h.mustDecode(&out)
	mustDeepEqual(t, out, in)
}

func TestNoPanicOnUnsupportedKey(t *testing.T) {
	data := []byte{0x81, 0x81, 0xa1, 0x78, 0xc3, 0xc3}
	_, err := msgpack.NewDecoder(bytes.NewReader(data)).DecodeTypedMap()
	mustErrorString(t, err, "msgpack: unsupported map key: map[string]interface {}")
}

func TestMapDefault(t *testing.T) {
	in := map[string]any{
		"foo": "bar",
		"hello": map[string]any{
			"foo": "bar",
		},
	}
	b, err := msgpack.Marshal(in)
	mustOK(t, err)
	var out map[string]any
	mustOK(t, msgpack.Unmarshal(b, &out))
	mustDeepEqual(t, out, in)
}

func TestRawMessage(t *testing.T) {
	type In struct {
		Foo map[string]any
	}
	type Out struct {
		Foo msgpack.RawMessage
	}
	type Out2 struct {
		Foo any
	}

	b, err := msgpack.Marshal(&In{
		Foo: map[string]any{"hello": "world"},
	})
	mustOK(t, err)

	var out Out
	mustOK(t, msgpack.Unmarshal(b, &out))

	var m map[string]string
	mustOK(t, msgpack.Unmarshal(out.Foo, &m))
	wantMap := map[string]string{"hello": "world"}
	mustDeepEqual(t, m, wantMap)

	msg := new(msgpack.RawMessage)
	out2 := Out2{Foo: msg}
	mustOK(t, msgpack.Unmarshal(b, &out2))
	mustBytesEqual(t, out.Foo, *msg)
}

func TestInterface(t *testing.T) {
	type Interface struct {
		Foo any
	}
	in := Interface{Foo: "foo"}
	b, err := msgpack.Marshal(in)
	mustOK(t, err)
	var str string
	out := Interface{Foo: &str}
	mustOK(t, msgpack.Unmarshal(b, &out))
	mustEqual(t, str, "foo")
}

func TestNaN(t *testing.T) {
	in := float64(math.NaN())
	b, err := msgpack.Marshal(in)
	mustOK(t, err)
	var out float64
	mustOK(t, msgpack.Unmarshal(b, &out))
	mustTrue(t, math.IsNaN(out), "expected NaN")
}

func TestSetSortMapKeys(t *testing.T) {
	in := map[string]any{
		"a": "a",
		"b": "b",
		"c": "c",
		"d": "d",
	}
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	enc.SetSortMapKeys(true)
	dec := msgpack.NewDecoder(&buf)

	mustOK(t, enc.Encode(in))
	wantWire := append([]byte(nil), buf.Bytes()...)
	buf.Reset()

	for range 100 {
		mustOK(t, enc.Encode(in))
		mustBytesEqual(t, buf.Bytes(), wantWire)
		out, err := dec.DecodeMap()
		mustOK(t, err)
		mustDeepEqual(t, out, in)
	}
}

func TestSetOmitEmpty(t *testing.T) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	enc.SetOmitEmpty(true)
	dec := msgpack.NewDecoder(&buf)

	t.Run("embedding_ptr_zero", func(t *testing.T) {
		mustOK(t, enc.Encode(EmbeddingPtrTest{}))
		var t2 *EmbeddingPtrTest
		mustOK(t, dec.Decode(&t2))
		if t2.Exported != nil {
			t.Fatal("expected nil Exported")
		}
	})

	type Nested struct {
		Foo string
		Bar string
	}
	type Item struct {
		X Nested
		Y *Nested
	}

	t.Run("omit_nested_zero", func(t *testing.T) {
		buf.Reset()
		mustOK(t, enc.Encode(Item{}))
		raw := buf.Bytes()
		mustTrue(t, !bytes.Contains(raw, []byte{'X'}), "did not expect field key 'X' in wire")
		mustTrue(t, !bytes.Contains(raw, []byte{'Y'}), "did not expect field key 'Y' in wire")
	})

	t.Run("emit_pointer_field", func(t *testing.T) {
		buf.Reset()
		mustOK(t, enc.Encode(Item{Y: &Nested{}}))
		raw := buf.Bytes()
		mustTrue(t, !bytes.Contains(raw, []byte{'X'}), "did not expect field key 'X' in wire")
		mustTrue(t, bytes.Contains(raw, []byte{'Y'}), "expected field key 'Y' in wire")
	})
}

type NullInt struct {
	Valid bool
	Int   int
}

func (i *NullInt) Set(j int) {
	i.Int = j
	i.Valid = true
}

func (i NullInt) IsZero() bool {
	return !i.Valid
}

func (i NullInt) MarshalMsgpack() ([]byte, error) {
	return msgpack.Marshal(i.Int)
}

func (i *NullInt) UnmarshalMsgpack(b []byte) error {
	if err := msgpack.Unmarshal(b, &i.Int); err != nil {
		return err
	}
	i.Valid = true
	return nil
}

type Secretive struct {
	Visible bool
	hidden  bool
}

type T struct {
	I NullInt `msgpack:",omitempty"`
	J NullInt
	// Secretive is not a "simple" struct because it has an hidden field.
	S Secretive `msgpack:",omitempty"`
}

func ExampleMarshal_ignore_simple_zero_structs_when_tagged_with_omitempty() {
	var t1 T
	raw, err := msgpack.Marshal(t1)
	if err != nil {
		panic(err)
	}
	var t2 T
	if err = msgpack.Unmarshal(raw, &t2); err != nil {
		panic(err)
	}
	fmt.Printf("%#v\n", t2)

	t2.I.Set(42)
	t2.S.hidden = true // won't be included because it is a hidden field
	raw, err = msgpack.Marshal(t2)
	if err != nil {
		panic(err)
	}
	var t3 T
	if err = msgpack.Unmarshal(raw, &t3); err != nil {
		panic(err)
	}
	fmt.Printf("%#v\n", t3)
	// Output: msgpack_test.T{I:msgpack_test.NullInt{Valid:false, Int:0}, J:msgpack_test.NullInt{Valid:true, Int:0}, S:msgpack_test.Secretive{Visible:false, hidden:false}}
	// msgpack_test.T{I:msgpack_test.NullInt{Valid:true, Int:42}, J:msgpack_test.NullInt{Valid:true, Int:0}, S:msgpack_test.Secretive{Visible:false, hidden:false}}
}

type (
	Value   any
	Wrapper struct {
		Value Value `msgpack:"v,omitempty"`
	}
)

func TestEncodeWrappedValue(t *testing.T) {
	v := (*time.Time)(nil)
	c := &Wrapper{Value: v}
	var buf bytes.Buffer
	mustOK(t, msgpack.NewEncoder(&buf).Encode(v))
	mustOK(t, msgpack.NewEncoder(&buf).Encode(c))
}

func TestPtrValueDecode(t *testing.T) {
	type Foo struct {
		Bar *int
	}
	b, err := msgpack.Marshal(Foo{})
	mustOK(t, err)

	bar1 := 123
	foo := Foo{Bar: &bar1}
	mustOK(t, msgpack.Unmarshal(b, &foo))
	if foo.Bar != nil {
		t.Fatal("expected nil Bar")
	}

	bar2 := 456
	b, err = msgpack.Marshal(Foo{Bar: &bar2})
	mustOK(t, err)
	mustOK(t, msgpack.Unmarshal(b, &foo))
	if foo.Bar == nil {
		t.Fatal("expected non-nil Bar")
	}
	mustEqual(t, *foo.Bar, bar2)
}
