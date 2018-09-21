package bson

import "testing"

func TestDecodeRawCap(t *testing.T) {
	var v struct{ R *Raw }
	marshalUnmarshal(t, struct {
		R int
	}{R: 1}, &v)
	if len(v.R.Data) != cap(v.R.Data) {
		t.Fatalf("expected len == cap, got len = %d and cap = %d", len(v.R.Data), cap(v.R.Data))
	}
}

func TestDecodeBytesCap(t *testing.T) {
	v := struct{ B []byte }{[]byte{1}}
	marshalUnmarshal(t, v, &v)
	if len(v.B) != cap(v.B) {
		t.Fatalf("expected len == cap, got len = %d and cap = %d", len(v.B), cap(v.B))
	}
}

func marshalUnmarshal(t *testing.T, in, out interface{}) {
	data, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
}
