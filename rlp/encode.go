package rlp

import (
	"reflect"
	"math/big"
	"errors"
	"fmt"
	"io"
)

func Encode(v interface{}) ([]byte, error) {
	println(fmt.Sprintf("Encode: %v", v))
	// Before marshaling, determine the length of the final array
	item, err := getItem(v)
	if err != nil {
		return nil, err
	}

	println("Size", item.size)

	data := make([]byte, 0, item.size)
	data = encodeItem(data, item)

	println(fmt.Sprintf("Result: %x\n", data))
	if len(data) != item.size {
		return nil, fmt.Errorf("final array should be %d bytes not %d", item.size, len(data))
	}

	return data, nil
}

type Item struct {
	size     int
	v        interface{}
	itemList []*Item
	dataSize int
}

type Encoder interface {
	EncodeRLP(io.Writer) error
}

var (
	encoderInterface = reflect.TypeOf(new(Encoder)).Elem()
	bigInt = reflect.TypeOf(big.Int{})
)

func getItem(v interface{}) (*Item, error) {
	item := &Item{ v: v }
	err := errors.New("")

	typ := reflect.TypeOf(v)
	kind := typ.Kind()
	switch {
	case typ.AssignableTo(reflect.PtrTo(bigInt)):
		if item.size, err = getIntPtr(v.(*big.Int)); err != nil {
			return nil, err
		}
	case typ.AssignableTo(bigInt):
		if item.size, err = getInt(v.(big.Int)); err != nil {
			return nil, err
		}
	case isUint(kind):
		item.size = getUint(reflect.ValueOf(v).Uint())
	case kind == reflect.String:
		if item.size, err = getString(v.(string)); err != nil {
			return nil, err
		}
	case kind == reflect.Bool:
		item.size = getBool()
	case kind == reflect.Slice && isByte(typ.Elem()):
		if item.size, err = getByteSlice(v.([]byte)); err != nil {
			return nil, err
		}
	case kind == reflect.Array && isByte(typ.Elem()):
		bytes := v.([]byte)
		if item.size, err = getString(string(bytes)); err != nil {
			return nil, err
		}
	case kind == reflect.Slice:
		if item, err = getSlice(v); err != nil {
			return nil, err
		}
	case kind == reflect.Struct:
		if item, err = getStruct(v); err != nil {
			return nil, err
		}
	case kind == reflect.Ptr:
		if item, err = getPtr(v); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("rlp: unsupported item type")
	}

	return item, nil
}

func getPtr(v interface{}) (*Item, error) {
	item := &Item{ v: v }

	val := reflect.ValueOf(v)
	if val.IsNil() {
		item.size = 1
		return item, nil
	}

	item, err := getItem(val.Elem().Interface())
	if err != nil {
		return nil, fmt.Errorf("cannot encode pointer: %v", err)
	}

	return item, nil
}

func getStruct(v interface{}) (*Item, error) {
	val := reflect.ValueOf(v)
	typ := val.Type()

	len := val.NumField()

	item := &Item{ 0, v, make([]*Item, 0, len), 0 }
	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		structF := typ.Field(i)

		arrayItem, err := getItem(f.Interface())
		if err != nil {
			return nil, fmt.Errorf("cannot encode struct %v: %v", structF.Name, err)
		}

		item.itemList = append(item.itemList, arrayItem)
		item.dataSize += arrayItem.size
	}

	listHeaderSize, err := getListHeaderSize(item.dataSize)
	if err != nil {
		return nil, fmt.Errorf("cannot encode struct: %v", err)
	}

	item.size = item.dataSize + listHeaderSize

	return item, nil
}

func getSlice(v interface{}) (*Item, error) {
	val := reflect.ValueOf(v)
	len := val.Len()

	item := &Item{ 0, v, make([]*Item, 0, len), 0 }

	for i := 0; i < len; i++ {
		arrayItem, err := getItem(val.Index(i).Interface())
		if err != nil {
			return nil, fmt.Errorf("err at index %d: %v", i, err)
		}

		item.dataSize += arrayItem.size
		item.itemList = append(item.itemList, arrayItem)
	}

	listHeaderSize, err := getListHeaderSize(item.dataSize)
	if err != nil {
		return nil, err
	}

	item.size = item.dataSize + listHeaderSize

	return item, nil
}

func getListHeaderSize(size int) (int, error) {
	if size < 56 {
		return 1, nil
	} else if encodedSize := getBigEndianSize(uint(size)); encodedSize > 9 {
		return 0, fmt.Errorf("encoding size exceeded limit: %d bytes long", size)
	} else {
		return encodedSize + 1, nil
	}
}

func getByteSlice(data []byte) (int, error) {
	return getString(string(data))
}

func getUint(data uint64) int {
	if data < 128 {
		return 1
	} else {
		return getBigEndianSize(uint(data)) + 1
	}
}

func getByteHeaderSize(data []byte) (int, error) {
	size := len(data)
	if size == 1 && data[0] <= 0x7F {
		return 0, nil
	} else if size < 56 {
		return 1, nil
	} else if encodedSize := getBigEndianSize(uint(size)); encodedSize > 9 {
		return 0, fmt.Errorf("encoding size exceeded limit: %d bytes long", size)
	} else {
		return encodedSize + 1, nil
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

func getIntPtr(v *big.Int) (int, error) {
	if v == nil {
		return 1, nil
	} else if sign := v.Sign(); sign == -1 {
		return 0, fmt.Errorf("rlp: cannot encode negative *big.Int")
	} else if sign == 0 {
		return 1, nil
	} else {
		intAsBytes := v.Bytes()
		byteHeaderSize, err := getByteHeaderSize(intAsBytes)
		if err != nil {
			return 0, err
		}

		return byteHeaderSize + len(intAsBytes), nil
	}

}

func getInt(v big.Int) (int, error) {
	return getIntPtr(&v)
}

func getString(v string) (int, error) {
	// TODO: Optimize this to avoid converting to a byte array
	byteHeaderSize, err := getByteHeaderSize([]byte(v))
	if err != nil {
		return 0, err
	}

	return byteHeaderSize + len(v), nil
}

func getBool() int {
	return 1
}

func encodeItem(data []byte, item *Item) []byte {
	typ := reflect.TypeOf(item.v)
	kind := typ.Kind()
	switch {
	case typ.AssignableTo(reflect.PtrTo(bigInt)):
		data = encodeIntPtr(data, item)
	case typ.AssignableTo(bigInt):
		data = encodeInt(data, item)
	case isUint(typ.Kind()):
		data = encodeUint(data, item)
	case typ.Kind() == reflect.String:
		data = encodeString(data, item)
	case typ.Kind() == reflect.Bool:
		data = encodeBool(data, item)
	case kind == reflect.Slice && isByte(typ.Elem()):
		data = encodeByteSlice(data, item)
	case kind == reflect.Array && isByte(typ.Elem()):
		data = encodeString(data, item)
	case kind == reflect.Slice:
		data = encodeSlice(data, item)
	case kind == reflect.Array:
	case kind == reflect.Struct:
		data = encodeStruct(data, item)
	case kind == reflect.Ptr:
		data = encodePtr(data, item)
	}

	return data
}

func encodePtr(data []byte, item *Item) []byte {
	val := reflect.ValueOf(item.v)
	if val.IsNil() {
		typ := reflect.TypeOf(item.v).Elem()
		kind := typ.Kind()

		if kind == reflect.Array && isByte(typ.Elem()) {
			return append(data, 0x80)
		} else if kind == reflect.Struct || kind == reflect.Array {
			return append(data, 0xc0)
		} else {
			// TODO: fix this

			return append(data, 0x80)
		}
	}

	panic("This shouldn't happen")
}

func encodeSlice(data []byte, item *Item) []byte {
	data = encodeListHeader(data, item.dataSize)

	for i := 0; i < len(item.itemList); i++ {
		data = encodeItem(data, item.itemList[i])
	}

	return data
}

func encodeStruct(data []byte, item *Item) []byte {
	return encodeSlice(data, item)
}

func encodeListHeader(data []byte, size int) []byte {
	if size < 56 {
		return append(data, 0xc0+byte(size))
	} else {
		byteHeader := convertBigEndian(uint(size))
		data = append(data, 0xf7 + byte(len(byteHeader)))
		return append(data, byteHeader...)
	}
	return nil
}

func encodeByteSlice(data []byte, item *Item) []byte {
	v := item.v.([]byte)

	if len(v) == 1 {
		return encodeByte(data, v[0])
	} else {
		return encodeBytes(data, v)
	}
}

func encodeString(data []byte, item *Item) []byte {
	v := item.v.(string)
	if len(v) == 1 {
		return encodeByte(data, v[0])
	} else {
		data = encodeByteHeader(data, len(v))
		return append(data, v...)
	}
}

func encodeUint(data []byte, item *Item) []byte {
	v := reflect.ValueOf(item.v).Uint()
	if v == 0 {
		return append(data, 0x80)
	} else if v < 128 {
		return encodeByte(data, byte(v))
	} else {
		b := convertBigEndian(uint(v))
		return encodeBytes(data, b)
	}
}

func encodeIntPtr(data []byte, item *Item) []byte {
	v := item.v.(*big.Int)
	if v == nil {
		return append(data, 0x80)
	}
	return encodeBigInt(data, v)
}

func encodeInt(data []byte, item *Item) []byte {
	v := item.v.(big.Int)
	return encodeBigInt(data, &v)
}

func encodeBigInt(data []byte, v *big.Int) []byte {
	if sign := v.Sign(); sign < 0 {
		panic("rlp: can not encode negative big.Int")
	} else if sign == 0 {
		return append(data, 0x80)
	} else if vb := v.Bytes(); len(vb) == 1 {
		return encodeByte(data, vb[0])
	} else {
		return encodeBytes(data, vb)
	}
}

func encodeBool(data []byte, item *Item) []byte {
	v := item.v.(bool)
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
		byteHeader := convertBigEndian(uint(size))
		data = append(data, 0xb7 + byte(len(byteHeader)))
		data = append(data, byteHeader...)
		return data
	}
}

func convertBigEndian(num uint) []byte {
	data := make([]byte, 0)

	for ; num >= 1; num = num >> 8 {
		data = append([]byte{ byte(num) }, data...)
	}

	return data
}

func isUint(k reflect.Kind) bool {
	return k >= reflect.Uint && k <= reflect.Uintptr
}

func isByte(typ reflect.Type) bool {
	return typ.Kind() == reflect.Uint8 && !typ.Implements(encoderInterface)
}
