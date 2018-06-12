package rlp

import (
	"reflect"
	"math/big"
	"errors"
	"fmt"
)

func Encode(v interface{}) ([]byte, error) {
	// Before marshaling, determine the length of the final array
	item, err := getItem(v)
	if err != nil {
		return nil, err
	}

	data := encodeItem(item)

	return data, nil
}

type Item struct {
	size     int
	v        interface{}
	itemList []*Item
}

var (
	bigInt = reflect.TypeOf(big.Int{})
	big0   = big.NewInt(0)
)

func getItem(v interface{}) (*Item, error) {
	item := Item{ v: v }
	err := errors.New("")

	typeOf := reflect.TypeOf(v)
	switch {
	case typeOf.Kind() == reflect.Array:
		if item.size, item.itemList, err = getArray(v.([]interface{})); err != nil {
			return nil, err
		}
	case isUint(typeOf.Kind()):
		item.size = getUint(v.(uint))
	case typeOf.Kind() == reflect.String:
		item.size = getString(v.(string))
	case typeOf.AssignableTo(bigInt):
		if item.size, err = getInt(v.(big.Int)); err != nil {
			return nil, err
		}
	case typeOf.Kind() == reflect.Bool:
		println("Detected a bool")
		item.size = getBool()
	default:
		return nil, fmt.Errorf("rlp: unsupported item type")
	}

	return &item, nil
}

func getUint(data uint) int {
	if data < 128 {
		return 1
	} else {
		return getBigEndianSize(data)

	}
}

func getByteHeaderSize(data []byte) int {
	if len(data) == 1 && data[0] <= 0x7F {
		return 0
	} else if len(data) < 56 {
		return 1
	} else {
		return getBigEndianSize(uint(len(data))) + 1
	}
}

func getBigEndianSize(num uint) int {
	size := uint(1)

	for ; num > 1 <<size; size++ {}

	return int(size)
}

func getArray(v []interface{}) (int, []*Item, error) {
	size := 0
	itemList := make([]*Item, len(v))

	for i := 0; i < len(v); i++ {
		arrayItem, err := getItem(v[i])
		if err != nil {
			return 0, nil, err
		}

		size += arrayItem.size
		itemList[i] = arrayItem
	}

	return size, itemList, nil
}

func getInt(v big.Int) (int, error) {
	if cmp := v.Cmp(big0); cmp == -1 {
		return 0, fmt.Errorf("rlp: cannot encode negative *big.Int")
	} else if cmp == 0 {
		return 1, nil
	} else {
		return getByteHeaderSize(v.Bytes()) + len(v.Bytes()), nil
	}
}

func getString(v string) int {
	return getByteHeaderSize([]byte(v)) + len(v)
}

func getBool() int {
	return 1
}

func encodeItem(item *Item) []byte {
	data := make([]byte, 0, item.size)

	typeOf := reflect.TypeOf(item.v)
	switch {
	case typeOf.Kind() == reflect.Array:
	case typeOf.Kind() == reflect.String:
		data = encodeString(data, item.v.(string))
	case isUint(typeOf.Kind()):
		// TODO: encode uint
	case typeOf.AssignableTo(bigInt):
		data = encodeInt(data, item.v.(big.Int))
	case typeOf.Kind() == reflect.Bool:
		data = encodeBool(data, item.v.(bool))
	}
	println("data length", len(data))

	return data
}

func encodeString(data []byte, v string) []byte {
	if len(v) == 1 {
		return encodeByte(data, v[0])
	} else {
		return encodeBytes(data, []byte(v))
	}
}

func encodeInt(data []byte, v big.Int) []byte {
	if cmp := v.Cmp(big0); cmp == -1 {
		panic("rlp: can not encode negative big.Int")
	} else if cmp == 0 {
		return encodeByte(data, 0x80)
	} else {
		return encodeBytes(data, v.Bytes())
	}
}

func encodeBool(data []byte, v bool) []byte {
	if v {
		return append(data, 0x01)
	} else {
		return append(data, 0x80)
	}
}

func encodeByte(data []byte, b byte) []byte {
	if b <= 0x7F {
		return append(data, b)
	} else {
		return append(data, 0x81, b)
	}
}

func encodeBytes(data []byte, b []byte) []byte {
	data = encodeByteHeader(data, len(b))
	return append(data, b...)
}

func encodeByteHeader(data []byte, size int) []byte {
	if size < 56 {
		return append(data, 0x80+byte(size))
	} else {
		byteHeader := convertBigEndian(size)
		data = append(data, 0xb7 + byte(len(byteHeader)))
		data = append(data, byteHeader...)
		return data
	}
}

func convertBigEndian(size int) []byte {
	data := make([]byte, 0, 8)
	// TODO: confirm if necessary to convert to uint64
	usize := uint(size)

	for i := uint(0); usize > 1 << i; i += 8 {
		data = append(data, byte(i >> i))
	}

	size = len(data)
	for i := 0; i < len(data) / 2; i++ {
		j := size-i-1
		data[i], data[j] = data[j], data[i]
	}

	return data
}

func isUint(k reflect.Kind) bool {
	return k >= reflect.Uint && k <= reflect.Uintptr
}
