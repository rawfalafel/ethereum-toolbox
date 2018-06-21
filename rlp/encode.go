package rlp

import (
	"reflect"
	"math/big"
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

	return encodeItem(w, item)
}

type Item struct {
	size       int
	v          interface{}
	itemList   []*Item
	dataSize   int
	w          *writer
	encode
}

type Encoder interface {
	EncodeRLP(io.Writer) error
}

var (
	encoderInterface = reflect.TypeOf(new(Encoder)).Elem()
	bigInt = reflect.TypeOf(big.Int{})
	bigIntPtr = reflect.PtrTo(bigInt)
)

var implementsCache = map[reflect.Type]int{}

func getItem(v interface{}) (*Item, error) {
	var err error = nil
	var item *Item = nil

	typ := reflect.TypeOf(v)
	if typ == nil {
		return &Item{ v: v, size: 1 }, nil
	}

	kind := typ.Kind()
	switch {
	case implementsEncoder(typ):
		item, err = getEncoder(v)
	case typ.AssignableTo(bigIntPtr):
		item, err = getIntPtr(v.(*big.Int))
	case typ.AssignableTo(bigInt):
		item, err = getInt(v.(big.Int))
	case isUint(kind):
		item = getUint(v)
	case kind == reflect.String:
		item, err = getString(v)
	case kind == reflect.Bool:
		item = getBool(v)
	case kind == reflect.Slice && isByte(typ.Elem()):
		item, err = getByteSlice(v)
	case kind == reflect.Array && isByte(typ.Elem()):
		item, err = getByteArray(v)
	case kind == reflect.Slice || kind == reflect.Array:
		item, err = getSlice(v)
	case kind == reflect.Struct:
		item, err = getStruct(v)
	case kind == reflect.Ptr:
		item, err = getPtr(v)
	default:
		return nil, fmt.Errorf("rlp: unsupported item type: %v", kind)
	}

	return item, err
}

func implementsEncoder(typ reflect.Type) bool {
	v := implementsCache[typ]
	if v == 0 {
		if typ.Implements(encoderInterface) {
			v = 2
		} else {
			v = 1
		}
		implementsCache[typ] = v
	}

	return v == 2
}

func getByteArray(v interface{}) (*Item, error) {
	item := &Item{ v: v, encode: encodeByteArray }

	val := reflect.ValueOf(v)
	len := val.Len()
	if len == 0 && val.Index(0).Interface().(byte) <= 0x7F {
		item.size = 1
	} else if len < 56 {
		item.size = 1 + len
	} else if encodedSize := getBigEndianSize(uint(len)); encodedSize > 9 {
		return nil, fmt.Errorf("encoding size exceeded limit: %d bytes long", len)
	} else {
		item.size = 1 + encodedSize + len
	}

	return item, nil
}

func getEncoder(v interface{}) (*Item, error) {
	item := &Item{ v: v, w: newWriter(0) }
	if err := v.(Encoder).EncodeRLP(item.w); err != nil {
		return nil, err
	}
	item.size = len(item.w.data)
	item.encode = encodeEncoder

	return item, nil
}

func getPtr(v interface{}) (*Item, error) {
	item := &Item{ v: v, encode: encodePtr }

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

type fieldInfo struct {
	name     string
	exported bool
	tags     tags
}

var structCache = map[reflect.Type][]*fieldInfo{}

func getFieldInfo(typ reflect.Type, val reflect.Value) ([]*fieldInfo, error) {
	var err error
	fs := structCache[typ]
	if fs == nil {
		fs, err = getFieldInfo1(typ, val)
		if err != nil {
			return nil, err
		}
		structCache[typ] = fs
	}

	return fs, nil
}

func getFieldInfo1(typ reflect.Type, val reflect.Value) ([]*fieldInfo, error) {
	len := val.NumField()

	fs := make([]*fieldInfo, 0, len)
	for i := 0; i < len; i++ {
		structF := typ.Field(i)

		tags, err := parseStructTag(typ, i)
		if err != nil {
			return nil, err
		}

		f := &fieldInfo{ name: structF.Name, exported: structF.PkgPath == "", tags: tags }
		fs = append(fs, f)
	}

	return fs, nil
}

func getStruct(v interface{}) (*Item, error) {
	val := reflect.ValueOf(v)
	typ := val.Type()

	fs, err := getFieldInfo(typ, val)
	if err != nil {
		return nil, err
	}

	len := len(fs)
	item := &Item{ v: v, itemList: make([]*Item, 0, len), encode: encodeStruct }
	for i := 0; i < len; i++ {
		f := val.Field(i)

		// ignore unexported fields
		if !fs[i].exported {
			continue
		}

		if fs[i].tags.ignored {
			continue
		}

		arrayItem, err := getItem(f.Interface())
		if err != nil {
			return nil, fmt.Errorf("cannot encode struct %v: %v", fs[i].name, err)
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

	item := &Item{  v: v, itemList: make([]*Item, 0, len), encode: encodeSlice }

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

func getByteSlice(v interface{}) (*Item, error) {
	bytes := reflect.ValueOf(v).Bytes()
	item := &Item{ v: bytes, encode: encodeByteSlice }

	size, err := getByteHeaderSize(bytes)
	if err != nil {
		return nil, err
	}

	item.size = size + len(bytes)
	return item, nil
}

func getUint(v interface{}) *Item {
	vAsUint := reflect.ValueOf(v).Uint()
	item := &Item{ v: vAsUint, encode: encodeUint }

	if vAsUint < 128 {
		item.size = 1
	} else {
		item.size = getBigEndianSize(uint(vAsUint)) + 1
	}

	return item
}

func getStringHeaderSize(data string) (int, error) {
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

func getIntPtr(v *big.Int) (*Item, error) {
	item := &Item{ v: v, encode: encodeIntPtr }

	if v == nil {
		item.size = 1
	} else if sign := v.Sign(); sign == -1 {
		return nil, fmt.Errorf("rlp: cannot encode negative *big.Int")
	} else if sign == 0 {
		item.size = 1
	} else {
		intAsBytes := v.Bytes()
		byteHeaderSize, err := getByteHeaderSize(intAsBytes)
		if err != nil {
			return nil, err
		}

		item.size = byteHeaderSize + len(intAsBytes)
	}

	return item, nil
}

func getInt(v big.Int) (*Item, error) {
	item, err := getIntPtr(&v)
	item.encode = encodeInt
	return item, err
}

func getString(v interface{}) (*Item, error) {
	str := reflect.ValueOf(v).String()
	item := &Item{ v: str, encode: encodeString }

	byteHeaderSize, err := getStringHeaderSize(str)
	if err != nil {
		return nil, err
	}

	item.size = byteHeaderSize + len(str)

	return item, nil
}

func getBool(v interface{}) *Item {
	return &Item{ v: 1, size: 1, encode: encodeBool }
}

func encodeItem(w io.Writer, item *Item) error {
	typ := reflect.TypeOf(item.v)
	if typ == nil {
		return writeByte(w, 0xc0)
	}

	return item.encode(w, item)
}

type encode func(w io.Writer, item *Item) error

func encodeByteArray(w io.Writer, item *Item) error {
	val := reflect.ValueOf(item.v)
	if !val.CanAddr() {
		// Slice requires the value to be addressable.
		// Make it addressable by copying.
		copy := reflect.New(val.Type()).Elem()
		copy.Set(val)
		val = copy
	}
	size := val.Len()
	slice := val.Slice(0, size).Bytes()
	return encodeBytes(w, slice)
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
	v := item.v.(uint64)
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
