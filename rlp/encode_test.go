package rlp

import (
	"testing"
	"bytes"
	"encoding/hex"
	"strings"
	"fmt"
)

func TestEncodeBool(t *testing.T) {
	data, err := Encode(true)
	if err != nil {
		t.Errorf("Encode returned an error: %v", err)
	}

	answer := "01"
	if !bytes.Equal(data, unhex(answer)) {
		t.Errorf("Expected %v, got %v", answer, data)
	}
}

func TestEncodeString(t *testing.T) {
	data, err := Encode("dog")
	if err != nil {
		t.Errorf("Encode returned an error: %v", err)
	}

	answer := "83646F67"
	if !bytes.Equal(data, unhex(answer)) {
		t.Errorf("Expected %v, got %v", answer, data)
	}
}

func unhex(str string) []byte {
	b, err := hex.DecodeString(strings.Replace(str, " ", "", -1))
	if err != nil {
		panic(fmt.Sprintf("invalid hex string: %q", str))
	}
	return b
}
