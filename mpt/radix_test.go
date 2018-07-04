package mpt

import (
	"reflect"
	"testing"
)

func TestUpdate(t *testing.T) {
	node := NewPatriciaNode(Empty)
	node.update("do", "verb")

	val, err := node.getValue("do")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
	}

	if val != "verb" {
		t.Errorf("failed to retrieve correct value: %v", val)
	}

	node.update("dog", "puppy")
	val, err = node.getValue("dog")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
	}

	if val != "puppy" {
		t.Errorf("failed to retrieve correct value: %v", val)
	}
	// node.update("doge", "coin")
	// node.update("horse", "stallion")
}

func TestConvertPathToHex(t *testing.T) {
	hexPath := convertPathToHex("do")
	answer := []int{6, 4, 6, 15}
	if !reflect.DeepEqual(hexPath, answer) {
		t.Errorf("incorrect hex encoding: output %v should equal %v", hexPath, answer)
	}

	hexPath = convertPathToHex("horse")
	answer = []int{6, 8, 6, 15, 7, 2, 7, 3, 6, 5}
	if !reflect.DeepEqual(hexPath, answer) {
		t.Errorf("incorrect hex encoding: output %v should equal %v", hexPath, answer)
	}
}
