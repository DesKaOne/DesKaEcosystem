package crypto

import (
	"bytes"
	"testing"
)

func TestBase58RoundTrip(t *testing.T) {
	cases := [][]byte{nil, {0}, {0, 0, 1}, []byte("IndoChain"), bytes.Repeat([]byte{0xab}, 32)}
	for _, input := range cases {
		encoded := Base58Encode(input)
		decoded, err := Base58Decode(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(decoded, input) {
			t.Fatalf("round trip mismatch: %x != %x", decoded, input)
		}
	}
}

func TestAddressRoundTrip(t *testing.T) {
	codec := AddressCodec{Prefix: "iND", Version: 1}
	payload := bytes.Repeat([]byte{0x11}, 20)
	address := codec.Encode(payload)
	if len(address) == 0 || address[:3] != "iND" {
		t.Fatalf("unexpected address: %s", address)
	}
	decoded, err := codec.Decode(address)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Fatalf("payload mismatch")
	}
}

func TestAddressRejectsTampering(t *testing.T) {
	codec := AddressCodec{Prefix: "IND", Version: 1}
	address := codec.Encode(bytes.Repeat([]byte{0x22}, 20))
	last := address[len(address)-1]
	replacement := byte('1')
	if last == replacement {
		replacement = '2'
	}
	tampered := address[:len(address)-1] + string(replacement)
	if _, err := codec.Decode(tampered); err == nil {
		t.Fatal("tampered address unexpectedly accepted")
	}
}
