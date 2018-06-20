package rlp

import (
	"reflect"
	"math/big"
	"errors"
	"fmt"
	"io"
	"strings"
)

type writer struct {
	data []byte
}

func newWriter(size int) *writer {
	return &writer{ make([]byte, 0, size) }
}

func (w *writer) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func EncodeToBytes(v interface{}) ([]byte, error) {
	// Before marshaling, determine the length of the final array
	item, err := getItem(v)
	if err != nil {
		return nil, err
	}

	w := newWriter(item.size)
	if err = encodeItem(w, item); err != nil {
		return nil, err
	}

	if len(w.data) != item.size {
		return nil, fmt.Errorf("final array should be %d bytes not %d", item.size, len(w.data))
	}

	return w.data, nil
}

func Encode(w io.Writer, v interface{}) error {
	item, err := getItem(v)
	if err != nil {
		return err
	}

	encodeItem(w, item)
	return nil
}

type Item struct {
	size       int
	v          interface{}
	itemList   []*Item
	dataSize   int
	w          *writer
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
	if typ == nil {
		item.size = 1
		return item, nil
	}

	kind := typ.Kind()
	switch {
	case typ.Implements(encoderInterface):
		if item, err = getEncoder(v); err != nil {
			return nil, err
		}
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

func getEncoder(v interface{}) (*Item, error) {
	item := &Item{ v: v, w: newWriter(0) }
	if err := v.(Encoder).EncodeRLP(item.w); err != nil {
		return nil, err
	}
	item.size = len(item.w.data)

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

	item := &Item{ v: v, itemList: make([]*Item, 0, len) }
	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		structF := typ.Field(i)

		// ignore unexported fields
		if structF.PkgPath != "" {
			continue
		}

		tags, err := parseStructTag(typ, i)
		if err != nil {
			return nil, err
		}
		if tags.ignored {
			continue
		}

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

type tags struct {
	// rlp:"nil" controls whether empty input results in a nil pointer.
	nilOK bool
	// rlp:"tail" controls whether this field swallows additional list
	// elements. It can only be set for the last field, which must be
	// of slice type.
	tail bool
	// rlp:"-" ignores fields.
	ignored bool
}

func parseStructTag(typ reflect.Type, fi int) (tags, error) {
	f := typ.Field(fi)
	var ts tags
	for _, t := range strings.Split(f.Tag.Get("rlp"), ",") {
		switch t = strings.TrimSpace(t); t {
		case "":
		case "-":
			ts.ignored = true
		case "nil":
			ts.nilOK = true
		case "tail":
			ts.tail = true
			if fi != typ.NumField()-1 {
				return ts, fmt.Errorf(`rlp: invalid struct tag "tail" for %v.%s (must be on last field)`, typ, f.Name)
			}
			if f.Type.Kind() != reflect.Slice {
				return ts, fmt.Errorf(`rlp: invalid struct tag "tail" for %v.%s (field type is not slice)`, typ, f.Name)
			}
		default:
			return ts, fmt.Errorf("rlp: unknown struct tag %q on %v.%s", t, typ, f.Name)
		}
	}
	return ts, nil
}

func getSlice(v interface{}) (*Item, error) {
	val := reflect.ValueOf(v)
	len := val.Len()

	item := &Item{  v: v, itemList: make([]*Item, 0, len) }

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

func encodeItem(w io.Writer, item *Item) error {
	typ := reflect.TypeOf(item.v)
	if typ == nil {
		return writeByte(w, 0xc0)
	}

	kind := typ.Kind()
	switch {
	case typ.Implements(encoderInterface):
		return encodeEncoder(w, item)
	case typ.AssignableTo(reflect.PtrTo(bigInt)):
		return encodeIntPtr(w, item)
	case typ.AssignableTo(bigInt):
		return encodeInt(w, item)
	case isUint(typ.Kind()):
		return encodeUint(w, item)
	case typ.Kind() == reflect.String:
		return encodeString(w, item)
	case typ.Kind() == reflect.Bool:
		return encodeBool(w, item)
	case kind == reflect.Slice && isByte(typ.Elem()):
		return encodeByteSlice(w, item)
	case kind == reflect.Array && isByte(typ.Elem()):
		return encodeString(w, item)
	case kind == reflect.Slice:
		return encodeSlice(w, item)
	case kind == reflect.Array:
	case kind == reflect.Struct:
		return encodeStruct(w, item)
	case kind == reflect.Ptr:
		return encodePtr(w, item)
	}

	panic("This should never happen")
}

func encodeEncoder(w io.Writer, item *Item) error {
	return writeBytes(w, item.w.data)
}

func encodePtr(w io.Writer, item *Item) error {
	val := reflect.ValueOf(item.v)
	if val.IsNil() {
		typ := reflect.TypeOf(item.v).Elem()
		kind := typ.Kind()

		if kind == reflect.Array && isByte(typ.Elem()) {
			return writeByte(w, 0x80)
		} else if kind == reflect.Struct || kind == reflect.Array {
			return writeByte(w, 0xc0)
		} else {
			v := reflect.Zero(typ).Interface()
			item, _ := getItem(v)
			return encodeItem(w, item)
		}
	}

	panic("This shouldn't happen")
}

func encodeSlice(w io.Writer, item *Item) error {
	if err := encodeListHeader(w, item.dataSize); err != nil {
		return err
	}

	for i := 0; i < len(item.itemList); i++ {
		if err := encodeItem(w, item.itemList[i]); err != nil {
			return err
		}
	}

	return nil
}

func encodeStruct(w io.Writer, item *Item) error {
	return encodeSlice(w, item)
}

func encodeListHeader(w io.Writer, size int) error {
	if size < 56 {
		return writeByte(w, 0xc0+byte(size))
	} else {
		byteHeader := convertBigEndian(uint(size))
		if err := writeByte(w, 0xf7 + byte(len(byteHeader))); err != nil {
			return err
		}

		return writeBytes(w, byteHeader)
	}
	return nil
}

func encodeByteSlice(w io.Writer, item *Item) error {
	v := item.v.([]byte)

	if len(v) == 1 {
		return encodeByte(w, v[0])
	} else {
		return encodeBytes(w, v)
	}
}

func encodeString(w io.Writer, item *Item) error {
	v := item.v.(string)
	if len(v) == 1 {
		return encodeByte(w, v[0])
	} else {
		if err := encodeByteHeader(w, len(v)); err != nil {
			return err
		}
		return writeBytes(w, []byte(v))
	}
}

func encodeUint(w io.Writer, item *Item) error {
	v := reflect.ValueOf(item.v).Uint()
	if v == 0 {
		return writeByte(w, 0x80)
	} else if v < 128 {
		return encodeByte(w, byte(v))
	} else {
		b := convertBigEndian(uint(v))
		return encodeBytes(w, b)
	}
}

func encodeIntPtr(w io.Writer, item *Item) error {
	v := item.v.(*big.Int)
	if v == nil {
		return writeByte(w, 0x80)
	}
	return encodeBigInt(w, v)
}

func encodeInt(w io.Writer, item *Item) error {
	v := item.v.(big.Int)
	return encodeBigInt(w, &v)
}

func encodeBigInt(w io.Writer, v *big.Int) error {
	if sign := v.Sign(); sign < 0 {
		panic("rlp: can not encode negative big.Int")
	} else if sign == 0 {
		return writeByte(w, 0x80)
	} else if vb := v.Bytes(); len(vb) == 1 {
		return encodeByte(w, vb[0])
	} else {
		return encodeBytes(w, vb)
	}
}

func encodeBool(w io.Writer, item *Item) error {
	v := item.v.(bool)
	if v {
		return writeByte(w, 0x01)
	} else {
		return writeByte(w, 0x80)
	}
}

func writeByte(w io.Writer, b byte) error {
	_, err := w.Write([]byte{b})
	return err
}

func writeBytes(w io.Writer, b []byte) error {
	_, err := w.Write(b)
	return err
}

func encodeByte(w io.Writer, b byte) error {
	if b <= 0x7F {
		return writeByte(w, b)
	} else {
		return writeBytes(w, []byte{0x81,b})
	}
}

func encodeBytes(w io.Writer, b []byte) error {
	if err := encodeByteHeader(w, len(b)); err != nil {
		return err
	}
	return writeBytes(w, b)
}

func encodeByteHeader(w io.Writer, size int) error {
	if size < 56 {
		return writeByte(w, 0x80+byte(size))
	} else {
		byteHeader := convertBigEndian(uint(size))
		if err := writeByte(w, 0xb7 + byte(len(byteHeader))); err != nil {
			return err
		}
		return writeBytes(w, byteHeader)
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
