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

	println("size", item.size)

	data := encodeItem(item)

	println(fmt.Sprintf("value: %v", v))
	println(fmt.Sprintf("data: %v\n", data))

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
	case isUint(typeOf.Kind()):
		item.size = getUint(reflect.ValueOf(v).Uint())
	case typeOf.Kind() == reflect.String:
		item.size = getString(v.(string))
	case typeOf.Kind() == reflect.Bool:
		item.size = getBool()
	default:
		return nil, fmt.Errorf("rlp: unsupported item type")
	}

	return &item, nil
}

func getUint(data uint64) int {
	if data < 128 {
		return 1
	} else {
		return getBigEndianSize(uint(data)) + 1
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
	i := uint(0)
	for ; num >= 1; i++ {
		num = num >> 8
	}

	return int(i)
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
	println("encodeItem")
	data := make([]byte, 0, item.size)

	typeOf := reflect.TypeOf(item.v)
	switch {
	case typeOf.Kind() == reflect.Array:
	case typeOf.Kind() == reflect.String:
		data = encodeString(data, item.v.(string))
	case isUint(typeOf.Kind()):
		data = encodeUint(data, reflect.ValueOf(item.v).Uint())
	case typeOf.AssignableTo(bigInt):
		data = encodeInt(data, item.v.(big.Int))
	case typeOf.Kind() == reflect.Bool:
		data = encodeBool(data, item.v.(bool))
	}

	return data
}

func encodeString(data []byte, v string) []byte {
	if len(v) == 1 {
		return encodeByte(data, v[0])
	} else {
		return encodeBytes(data, []byte(v))
	}
}

func encodeUint(data []byte, v uint64) []byte {
	if v == 0 {
		return append(data, 0x80)
	} else if v < 128 {
		return encodeByte(data, byte(v))
	} else {
		b := convertBigEndian(uint(v))
		return encodeBytes(data, b)
	}
}

func encodeInt(data []byte, v big.Int) []byte {
	if cmp := v.Cmp(big0); cmp == -1 {
		panic("rlp: can not encode negative big.Int")
	} else if cmp == 0 {
		return append(data, 0x80)
	} else {
		vb := v.Bytes()
		if len(vb) == 1 {
			return encodeByte(data, vb[0])
		} else {
			return encodeBytes(data, vb)
		}
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
	println("encodeByteHeader size", size)
	if size < 56 {
		return append(data, 0x80+byte(size))
	} else {
		byteHeader := convertBigEndian(uint(size))
		data = append(data, 0xb7 + byte(len(byteHeader)))
		data = append(data, byteHeader...)
		return data
	}
}

func convertBigEndian(num uint) []byte {
	println("convertBigEndian")
	data := make([]byte, 0)

	for ; num >= 1; num = num >> 8 {
		data = append([]byte{ byte(num) }, data...)
	}

	return data
}

func isUint(k reflect.Kind) bool {
	return k >= reflect.Uint && k <= reflect.Uintptr
}
