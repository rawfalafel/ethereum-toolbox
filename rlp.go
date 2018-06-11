package main

import (
	"reflect"
	"math/big"
	"fmt"
	"errors"
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
	case typeOf.AssignableTo(bigInt):
		if item.size, err = getInt(v.(big.Int)); err != nil {
			return nil, err
		}
	case typeOf.Kind() == reflect.Bool:
		item.size = getBool()
	default:
		return nil, fmt.Errorf("rlp: unsupported item type")
	}

	return &item, nil
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
		return len(v.Bytes()), nil
	}
}

func getBool() int {
	return 1
}

func encodeItem(item *Item) []byte {
	data := make([]byte, 0, item.size)

	typeOf := reflect.TypeOf(item)
	switch {
	case typeOf.Kind() == reflect.Array:

	case typeOf.AssignableTo(bigInt):
		data = encodeInt(data, item.v.(big.Int))
	case typeOf.Kind() == reflect.Bool:
		data = encodeBool(data, item.v.(bool))
	}

	return data
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
