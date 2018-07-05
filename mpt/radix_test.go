package mpt

import (
	"reflect"
	"testing"
)

func TestSimpleUpdate(t *testing.T) {
	node := NewPatriciaNode(Empty)
	_, err := node.update("do", "verb")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	val, err := node.getValue("do")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
		return
	}

	if val != "verb" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}
}

func TestExtensionConversion(t *testing.T) {
	node := NewPatriciaNode(Empty)
	_, err := node.update("do", "verb")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	_, err = node.update("dog", "puppy")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	val, err := node.getValue("dog")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
		return
	}

	if val != "puppy" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}

	val, err = node.getValue("do")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
		return
	}

	if val != "verb" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}
}

func TestExtensionConversion2(t *testing.T) {
	node := NewPatriciaNode(Empty)
	_, err := node.update("do", "verb")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	_, err = node.update("dp", "puppy")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	val, err := node.getValue("dp")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
		return
	}

	if val != "puppy" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}

	val, err = node.getValue("do")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
		return
	}

	if val != "verb" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}
}

func TestExtensionConversion3(t *testing.T) {
	node := NewPatriciaNode(Empty)
	_, err := node.update("do", "verb")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	_, err = node.update("dn", "puppy")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	val, err := node.getValue("dn")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
		return
	}

	if val != "puppy" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}

	val, err = node.getValue("do")
	if err != nil {
		t.Errorf("failed to retrieve string: %v", err)
		return
	}

	if val != "verb" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}
}

func TestExtensionConversionMixed(t *testing.T) {
	node := NewPatriciaNode(Empty)
	_, err := node.update("do", "verb")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	_, err = node.update("dog", "puppy")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	_, err = node.update("doge", "coin")
	if err != nil {
		t.Errorf("failed to update: %v", err)
		return
	}

	// _, err = node.update("horse", "stallion")
	// if err != nil {
	// 	t.Errorf("failed to update: %v", err)
	// 	return
	// }

	val, err := node.getValue("do")
	if err != nil {
		t.Errorf("failed to get: %v", err)
	}

	if val != "verb" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}

	val, err = node.getValue("dog")
	if err != nil {
		t.Errorf("failed to get: %v", err)
	}

	if val != "puppy" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}

	val, err = node.getValue("doge")
	if err != nil {
		t.Errorf("failed to get: %v", err)
	}

	if val != "coin" {
		t.Errorf("failed to retrieve correct value: %v", val)
		return
	}

	// val, err = node.getValue("horsse")
	// if err != nil {
	// 	t.Errorf("failed to get: %v", err)
	// }

	// if val != "stallion" {
	// 	t.Errorf("failed to retrieve correct value: %v", val)
	// 	return
	// }
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
