package msgpack_test

import (
	"bytes"
	"io"
	"testing"

	"quad4/msgpack/v5/pkg/msgpack"
)

type NoIntern struct {
	A string
	B string
	C any
}

type Intern struct {
	A string `msgpack:",intern"`
	B string `msgpack:",intern"`
	C any    `msgpack:",intern"`
}

func TestInternedString(t *testing.T) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	enc.UseInternedStrings(true)
	dec := msgpack.NewDecoder(&buf)
	dec.UseInternedStrings(true)

	for range 3 {
		mustOK(t, enc.EncodeString("hello"))
	}
	for range 3 {
		s, err := dec.DecodeString()
		mustOK(t, err)
		mustEqual(t, s, "hello")
	}

	mustOK(t, enc.Encode("hello"))
	v, err := dec.DecodeInterface()
	mustOK(t, err)
	mustEqual(t, v, "hello")

	_, err = dec.DecodeInterface()
	mustEqual(t, err, io.EOF)
}

func TestInternedStringTag(t *testing.T) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	dec := msgpack.NewDecoder(&buf)
	in := []Intern{
		{"f", "f", "f"},
		{"fo", "fo", "fo"},
		{"foo", "foo", "foo"},
		{"f", "fo", "foo"},
	}
	mustOK(t, enc.Encode(in))
	var out []Intern
	mustOK(t, dec.Decode(&out))
	mustDeepEqual(t, out, in)
}

func TestResetDict(t *testing.T) {
	dict := []string{"hello world", "foo bar"}
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	dec := msgpack.NewDecoder(&buf)

	t.Run("encode_string_with_dict", func(t *testing.T) {
		enc.ResetDict(&buf, dictMap(dict))
		mustOK(t, enc.EncodeString("hello world"))
		mustEqual(t, buf.Len(), 3)
		dec.ResetDict(&buf, dict)
		s, err := dec.DecodeString()
		mustOK(t, err)
		mustEqual(t, s, "hello world")
	})

	t.Run("encode_interface_with_dict", func(t *testing.T) {
		enc.ResetDict(&buf, dictMap(dict))
		mustOK(t, enc.Encode("foo bar"))
		mustEqual(t, buf.Len(), 3)
		dec.ResetDict(&buf, dict)
		s, err := dec.DecodeInterface()
		mustOK(t, err)
		mustEqual(t, s, "foo bar")
	})

	t.Run("non_dict_strings_expand_buffer", func(t *testing.T) {
		dec.ResetDict(&buf, dict)
		_ = enc.EncodeString("xxxx")
		mustEqual(t, buf.Len(), 5)
		_ = enc.Encode("xxxx")
		mustEqual(t, buf.Len(), 10)
	})
}

func TestMapWithInternedString(t *testing.T) {
	type M map[string]any
	dict := []string{"hello world", "foo bar"}
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(nil)
	enc.ResetDict(&buf, dictMap(dict))
	dec := msgpack.NewDecoder(nil)
	dec.ResetDict(&buf, dict)

	for range 100 {
		in := M{
			"foo bar":     "hello world",
			"hello world": "foo bar",
			"foo":         "bar",
		}
		mustOK(t, enc.Encode(in))
		_, err := dec.DecodeInterface()
		mustOK(t, err)
	}
}

func dictMap(dict []string) map[string]int {
	m := make(map[string]int, len(dict))
	for i, s := range dict {
		m[s] = i
	}
	return m
}
