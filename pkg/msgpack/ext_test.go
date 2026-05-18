package msgpack_test

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Quad4-Software/msgpack/v5/pkg/msgpack"
	"github.com/Quad4-Software/msgpack/v5/pkg/msgpack/msgpcode"
)

func init() {
	msgpack.RegisterExt(9, (*ExtTest)(nil))
}

type ExtTest struct {
	S string
}

var (
	_ msgpack.Marshaler   = (*ExtTest)(nil)
	_ msgpack.Unmarshaler = (*ExtTest)(nil)
)

func (ext ExtTest) MarshalMsgpack() ([]byte, error) {
	return msgpack.Marshal("hello " + ext.S)
}

func (ext *ExtTest) UnmarshalMsgpack(b []byte) error {
	return msgpack.Unmarshal(b, &ext.S)
}

func TestEncodeDecodeExtHeader(t *testing.T) {
	v := &ExtTest{"world"}
	payload, err := v.MarshalMsgpack()
	mustOK(t, err)

	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	mustOK(t, enc.EncodeExtHeader(9, len(payload)))
	_, err = buf.Write(payload)
	mustOK(t, err)

	var dst any
	mustOK(t, msgpack.Unmarshal(buf.Bytes(), &dst))
	v = dst.(*ExtTest)
	mustEqual(t, v.S, "hello world")

	dec := msgpack.NewDecoder(&buf)
	extID, extLen, err := dec.DecodeExtHeader()
	mustOK(t, err)
	mustEqual(t, extID, int8(9))
	mustEqual(t, extLen, len(payload))

	data := make([]byte, extLen)
	mustOK(t, dec.ReadFull(data))
	v = &ExtTest{}
	mustOK(t, v.UnmarshalMsgpack(data))
	mustEqual(t, v.S, "hello world")
}

func TestExt(t *testing.T) {
	v := &ExtTest{"world"}
	b, err := msgpack.Marshal(v)
	mustOK(t, err)

	var dst any
	mustOK(t, msgpack.Unmarshal(b, &dst))
	v, ok := dst.(*ExtTest)
	mustTrue(t, ok, "got wrong type from Unmarshal")
	mustEqual(t, v.S, "hello world")

	ext := new(ExtTest)
	mustOK(t, msgpack.Unmarshal(b, &ext))
	mustEqual(t, ext.S, "hello world")
}

func TestUnknownExt(t *testing.T) {
	b := []byte{byte(msgpcode.FixExt1), 2, 0}
	var dst any
	err := msgpack.Unmarshal(b, &dst)
	mustErrorString(t, err, "msgpack: unknown ext id=2")
}

func TestSliceOfTime(t *testing.T) {
	in := []any{time.Now()}
	b, err := msgpack.Marshal(in)
	mustOK(t, err)
	var out []any
	mustOK(t, msgpack.Unmarshal(b, &out))
	outTime := out[0].(time.Time)
	inTime := in[0].(time.Time)
	mustEqual(t, outTime.Unix(), inTime.Unix())
}

type customPayload struct {
	payload []byte
}

func (cp *customPayload) MarshalMsgpack() ([]byte, error) {
	return cp.payload, nil
}

func (cp *customPayload) UnmarshalMsgpack(b []byte) error {
	cp.payload = b
	return nil
}

func TestDecodeCustomPayload(t *testing.T) {
	b, err := hex.DecodeString("c70500c09eec3100")
	mustOK(t, err)
	msgpack.RegisterExt(0, (*customPayload)(nil))
	var cp *customPayload
	mustOK(t, msgpack.Unmarshal(b, &cp))
	mustEqual(t, hex.EncodeToString(cp.payload), "c09eec3100")
}
