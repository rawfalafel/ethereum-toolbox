package rlp

import (
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
)

// EncodeToBytes ...
func EncodeToBytes(v interface{}) ([]byte, error) {
	val := reflect.ValueOf(v)
	typ := val.Type()

	i := getInfo(typ)

	siz, err := i.s(val)
	if err != nil {
		return nil, fmt.Errorf("Sizing failed: %v", err)
	}

	bs := make([]byte, 0, siz)
	bs = i.w(val, bs)

	if len(bs) != siz {
		return nil, fmt.Errorf("Size doesn't match: %d but should be %d", len(bs), siz)
	}

	return bs, nil
}

// func Encode(w io.Writer, v interface{}) error {
// 	// IMPLEMENT
// }

var infoCache = map[reflect.Type]*encodeInfo{}

func getInfo(typ reflect.Type) *encodeInfo {
	ei, ok := infoCache[typ]

	if !ok {
		ei = &encodeInfo{}
		ei.populate(typ)
		infoCache[typ] = ei
	}

	return ei
}

type writerz struct {
	data []byte
}

func newWriter(size int) *writerz {
	return &writerz{make([]byte, 0, size)}
}

func (w *writerz) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

type sizer func(reflect.Value) (int, error)
type writer func(reflect.Value, []byte) []byte

type encodeInfo struct {
	typ reflect.Type
	s   sizer
	w   writer
}

func (ei *encodeInfo) populate(typ reflect.Type) {
	ei.typ = typ
	if typ == nil {
		ei.s, ei.w = nilSizer, nilWriter
		return
	}

	switch {
	case typ.Implements(encoderInterface):
		ei.s, ei.w = makeEncoderSizer(typ), makeEncoderWriter(typ)
	case typ.Kind() == reflect.String:
		ei.s, ei.w = stringSizer, stringWriter
	case typ.Kind() == reflect.Struct:
		s, w, _ := makeStructFuncs(typ)
		ei.s, ei.w = s, w
	}
}

func nilSizer(_ reflect.Value) (int, error) {
	return 1, nil
}

func nilWriter(_ reflect.Value, b []byte) []byte {
	return append(b, 0xc0)
}

func makeEncoderSizer(typ reflect.Type) sizer {
	return func(v reflect.Value) (int, error) {
		wz := newWriter(0)
		// Can this be the pointer to avoid unnecessary copy?
		if err := v.Interface().(Encoder).EncodeRLP(wz); err != nil {
			return 0, err
		}

		return len(wz.data), nil
	}
}

func makeEncoderWriter(typ reflect.Type) writer {
	return func(v reflect.Value, b []byte) []byte {
		wz := newWriter(0)

		if err := v.Interface().(Encoder).EncodeRLP(wz); err != nil {
			panic(fmt.Sprintf("EncodeRLP of encoder object failed: %v", err))
		}

		return append(b, wz.data...)
	}
}

func stringSizer(v reflect.Value) (int, error) {
	str := v.String()
	byteHeaderSize, err := getStringHeaderSize(str)
	if err != nil {
		return 0, err
	}

	return byteHeaderSize + len(str), nil
}

func stringWriter(v reflect.Value, b []byte) []byte {
	str := v.String()
	if len(str) == 1 {
		return encodeByte(b, str[0])
	}

	b = encodeByteHeader(b, len(str))
	return append(b, str...)
}

func makeStructFuncs(typ reflect.Type) (sizer, writer, error) {
	fs, err := getFieldInfo(typ)
	if err != nil {
		return nil, nil, err
	}

	sizeCache := map[reflect.Value]int{}

	return func (v reflect.Value) (int, error) {
		siz := 0
		for i := 0; i < len(fs); i++ {
			f := v.Field(i)

			fsiz, err := fs[i].s(f)
			if err != nil {
				return 0, fmt.Errorf("error with %v: %v", fs[i].name, err)
			}

			siz += fsiz
		}

		sizeCache[v] = siz

		headerSize, err := getListHeaderSize(siz)
		if err != nil {
			return 0, err
		}

		return siz + headerSize, nil
	}, 
	func(v reflect.Value, b []byte) []byte {
		siz := sizeCache[v] 
		delete(sizeCache, v)

		b = encodeListHeader(b, siz)
		for i := 0; i < len(fs); i++ {
			f := v.Field(i)

			b = fs[i].w(f, b)
		}

		return b
	},
	nil
}

// Encoder ...
type Encoder interface {
	EncodeRLP(io.Writer) error
}

var (
	encoderInterface = reflect.TypeOf(new(Encoder)).Elem()
	bigInt           = reflect.TypeOf(big.Int{})
	bigIntPtr        = reflect.PtrTo(bigInt)
)

// var implementsCache = map[reflect.Type]int{}

// func getItem(v interface{}) (*Item, error) {
// 	var err error = nil
// 	var item *Item = nil

// 	typ := reflect.TypeOf(v)
// 	if typ == nil {
// 		return &Item{v: v, size: 1}, nil
// 	}

// 	kind := typ.Kind()
// 	switch {
// 	case implementsEncoder(typ):
// 		item, err = getEncoder(v)
// 	case typ.AssignableTo(bigIntPtr):
// 		item, err = getIntPtr(v.(*big.Int))
// 	case typ.AssignableTo(bigInt):
// 		item, err = getInt(v.(big.Int))
// 	case isUint(kind):
// 		item = getUint(v)
// 	case kind == reflect.String:
// 		item, err = getString(v)
// 	case kind == reflect.Bool:
// 		item = getBool(v)
// 	case kind == reflect.Slice && isByte(typ.Elem()):
// 		item, err = getByteSlice(v)
// 	case kind == reflect.Array && isByte(typ.Elem()):
// 		item, err = getByteArray(v)
// 	case kind == reflect.Slice || kind == reflect.Array:
// 		item, err = getSlice(v)
// 	case kind == reflect.Struct:
// 		item, err = getStruct(v)
// 	case kind == reflect.Ptr:
// 		item, err = getPtr(v)
// 	default:
// 		return nil, fmt.Errorf("rlp: unsupported item type: %v", kind)
// 	}

// 	return item, err
// }

// func implementsEncoder(typ reflect.Type) bool {
// 	v := implementsCache[typ]
// 	if v == 0 {
// 		if typ.Implements(encoderInterface) {
// 			v = 2
// 		} else {
// 			v = 1
// 		}
// 		implementsCache[typ] = v
// 	}

// 	return v == 2
// }

// func getByteArray(v interface{}) (*Item, error) {
// 	item := &Item{v: v, encode: encodeByteArray}

// 	val := reflect.ValueOf(v)
// 	len := val.Len()
// 	if len == 0 && val.Index(0).Interface().(byte) <= 0x7F {
// 		item.size = 1
// 	} else if len < 56 {
// 		item.size = 1 + len
// 	} else if encodedSize := getBigEndianSize(uint(len)); encodedSize > 9 {
// 		return nil, fmt.Errorf("encoding size exceeded limit: %d bytes long", len)
// 	} else {
// 		item.size = 1 + encodedSize + len
// 	}

// 	return item, nil
// }

// func getEncoder(v interface{}) (*Item, error) {
// 	item := &Item{v: v, w: newWriter(0)}
// 	if err := v.(Encoder).EncodeRLP(item.w); err != nil {
// 		return nil, err
// 	}
// 	item.size = len(item.w.data)
// 	item.encode = encodeEncoder

// 	return item, nil
// }

// func getPtr(v interface{}) (*Item, error) {
// 	val := reflect.ValueOf(v)
// 	if val.IsNil() {
// 		return &Item{v: v, encode: encodePtr}, nil
// 	}

// 	item, err := getItem(val.Elem().Interface())
// 	if err != nil {
// 		return nil, fmt.Errorf("cannot encode pointer: %v", err)
// 	}

// 	return item, nil
// }

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
	s        sizer
	w        writer
}

func getFieldInfo(typ reflect.Type) ([]*fieldInfo, error) {
	len := typ.NumField()

	fs := make([]*fieldInfo, 0, len)
	for i := 0; i < len; i++ {
		structF := typ.Field(i)

		tags, err := parseStructTag(typ, i)
		if err != nil {
			return nil, err
		}

		if tags.ignored || structF.PkgPath != "" {
			continue
		}

		ei := getInfo(structF.Type)
		f := &fieldInfo{name: structF.Name, exported: structF.PkgPath == "", tags: tags, s: ei.s, w: ei.w}
		fs = append(fs, f)
	}

	return fs, nil
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

// func getSlice(v interface{}) (*Item, error) {
// 	val := reflect.ValueOf(v)
// 	len := val.Len()

// 	item := &Item{v: v, itemList: make([]*Item, 0, len), encode: encodeSlice}

// 	for i := 0; i < len; i++ {
// 		arrayItem, err := getItem(val.Index(i).Interface())
// 		if err != nil {
// 			return nil, fmt.Errorf("err at index %d: %v", i, err)
// 		}

// 		item.dataSize += arrayItem.size
// 		item.itemList = append(item.itemList, arrayItem)
// 	}

// 	listHeaderSize, err := getListHeaderSize(item.dataSize)
// 	if err != nil {
// 		return nil, err
// 	}

// 	item.size = item.dataSize + listHeaderSize

// 	return item, nil
// }

// func getStruct(v interface{}) (*Item, error) {
// 	val := reflect.ValueOf(v)
// 	typ := val.Type()

// 	fs, err := getFieldInfo(typ, val)
// 	if err != nil {
// 		return nil, err
// 	}

// 	len := len(fs)
// 	item := &Item{v: v, itemList: make([]*Item, 0, len), encode: encodeStruct}
// 	for i := 0; i < len; i++ {
// 		f := val.Field(i)

// 		// ignore unexported fields
// 		if !fs[i].exported {
// 			continue
// 		}

// 		if fs[i].tags.ignored {
// 			continue
// 		}

// 		arrayItem, err := getItem(f.Interface())
// 		if err != nil {
// 			return nil, fmt.Errorf("cannot encode struct %v: %v", fs[i].name, err)
// 		}

// 		item.itemList = append(item.itemList, arrayItem)
// 		item.dataSize += arrayItem.size
// 	}

// 	listHeaderSize, err := getListHeaderSize(item.dataSize)
// 	if err != nil {
// 		return nil, fmt.Errorf("cannot encode struct: %v", err)
// 	}

// 	item.size = item.dataSize + listHeaderSize

// 	return item, nil
// }

func getListHeaderSize(size int) (int, error) {
	if size < 56 {
		return 1, nil
	} else if encodedSize := getBigEndianSize(uint(size)); encodedSize > 9 {
		return 0, fmt.Errorf("encoding size exceeded limit: %d bytes long", size)
	} else {
		return encodedSize + 1, nil
	}
}

// func getByteSlice(v interface{}) (*Item, error) {
// 	bytes := reflect.ValueOf(v).Bytes()
// 	item := &Item{v: bytes, encode: encodeByteSlice}

// 	size, err := getByteHeaderSize(bytes)
// 	if err != nil {
// 		return nil, err
// 	}

// 	item.size = size + len(bytes)
// 	return item, nil
// }

// func getUint(v interface{}) *Item {
// 	vAsUint := reflect.ValueOf(v).Uint()
// 	item := &Item{v: vAsUint, encode: encodeUint}

// 	if vAsUint < 128 {
// 		item.size = 1
// 	} else {
// 		item.size = getBigEndianSize(uint(vAsUint)) + 1
// 	}

// 	return item
// }

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

// func getIntPtr(v *big.Int) (*Item, error) {
// 	item := &Item{v: v, encode: encodeIntPtr}

// 	if v == nil {
// 		item.size = 1
// 	} else if sign := v.Sign(); sign == -1 {
// 		return nil, fmt.Errorf("rlp: cannot encode negative *big.Int")
// 	} else if sign == 0 {
// 		item.size = 1
// 	} else {
// 		intAsBytes := v.Bytes()
// 		byteHeaderSize, err := getByteHeaderSize(intAsBytes)
// 		if err != nil {
// 			return nil, err
// 		}

// 		item.size = byteHeaderSize + len(intAsBytes)
// 	}

// 	return item, nil
// }

// func getInt(v big.Int) (*Item, error) {
// 	item, err := getIntPtr(&v)
// 	item.encode = encodeInt
// 	return item, err
// }

// func getString(v interface{}) (*Item, error) {
// 	// TODO: avoid creating a string object
// 	str := reflect.ValueOf(v).String()
// 	item := &Item{v: str, encode: encodeString}

// 	byteHeaderSize, err := getStringHeaderSize(str)
// 	if err != nil {
// 		return nil, err
// 	}

// 	item.size = byteHeaderSize + len(str)

// 	return item, nil
// }

// func getBool(v interface{}) *Item {
// 	return &Item{v: 1, size: 1, encode: encodeBool}
// }

// func encodeItem(w io.Writer, item *Item) error {
// 	typ := reflect.TypeOf(item.v)
// 	if typ == nil {
// 		return writeByte(w, 0xc0)
// 	}

// 	return item.encode(w, item)
// }

// type encode func(w io.Writer, item *Item) error

// func encodeByteArray(w io.Writer, item *Item) error {
// 	val := reflect.ValueOf(item.v)
// 	if !val.CanAddr() {
// 		// Slice requires the value to be addressable.
// 		// Make it addressable by copying.
// 		copy := reflect.New(val.Type()).Elem()
// 		copy.Set(val)
// 		val = copy
// 	}
// 	size := val.Len()
// 	slice := val.Slice(0, size).Bytes()
// 	return encodeBytes(w, slice)
// }

// func encodeEncoder(w io.Writer, item *Item) error {
// 	return writeBytes(w, item.w.data)
// }

// func encodePtr(w io.Writer, item *Item) error {
// 	val := reflect.ValueOf(item.v)
// 	if val.IsNil() {
// 		typ := reflect.TypeOf(item.v).Elem()
// 		kind := typ.Kind()

// 		if kind == reflect.Array && isByte(typ.Elem()) {
// 			return writeByte(w, 0x80)
// 		} else if kind == reflect.Struct || kind == reflect.Array {
// 			return writeByte(w, 0xc0)
// 		} else {
// 			v := reflect.Zero(typ).Interface()
// 			item, _ := getItem(v)
// 			return encodeItem(w, item)
// 		}
// 	}

// 	panic("This shouldn't happen")
// }

// func encodeSlice(w io.Writer, item *Item) error {
// 	if err := encodeListHeader(w, item.dataSize); err != nil {
// 		return err
// 	}

// 	for i := 0; i < len(item.itemList); i++ {
// 		if err := encodeItem(w, item.itemList[i]); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// func encodeStruct(w io.Writer, item *Item) error {
// 	return encodeSlice(w, item)
// }

// func encodeByteSlice(w io.Writer, item *Item) error {
// 	v := item.v.([]byte)

// 	if len(v) == 1 {
// 		return encodeByte(w, v[0])
// 	} else {
// 		return encodeBytes(w, v)
// 	}
// }

// func encodeString(w io.Writer, item *Item) error {
// 	v := item.v.(string)
// 	if len(v) == 1 {
// 		return encodeByte(w, v[0])
// 	} else {
// 		if err := encodeByteHeader(w, len(v)); err != nil {
// 			return err
// 		}
// 		return writeBytes(w, []byte(v))
// 	}
// }

// func encodeUint(w io.Writer, item *Item) error {
// 	v := item.v.(uint64)
// 	if v == 0 {
// 		return writeByte(w, 0x80)
// 	} else if v < 128 {
// 		return encodeByte(w, byte(v))
// 	} else {
// 		b := convertBigEndian(uint(v))
// 		return encodeBytes(w, b)
// 	}
// }

// func encodeIntPtr(w io.Writer, item *Item) error {
// 	v := item.v.(*big.Int)
// 	if v == nil {
// 		return writeByte(w, 0x80)
// 	}
// 	return encodeBigInt(w, v)
// }

// func encodeInt(w io.Writer, item *Item) error {
// 	v := item.v.(big.Int)
// 	return encodeBigInt(w, &v)
// }

// func encodeBigInt(w io.Writer, v *big.Int) error {
// 	if sign := v.Sign(); sign < 0 {
// 		panic("rlp: can not encode negative big.Int")
// 	} else if sign == 0 {
// 		return writeByte(w, 0x80)
// 	} else if vb := v.Bytes(); len(vb) == 1 {
// 		return encodeByte(w, vb[0])
// 	} else {
// 		return encodeBytes(w, vb)
// 	}
// }

// func encodeBool(w io.Writer, item *Item) error {
// 	v := item.v.(bool)
// 	if v {
// 		return writeByte(w, 0x01)
// 	} else {
// 		return writeByte(w, 0x80)
// 	}
// }

func encodeByte(bs []byte, b byte) []byte {
	if b <= 0x7F {
		return append(bs, b)
	}

	return append(bs, 0x81, b)
}

func encodeBytes(bs []byte, b []byte) []byte {
	bs = encodeByteHeader(bs, len(b))
	return append(bs, b...)
}

func encodeListHeader(bs []byte, size int) []byte {
	if size < 56 {
		return append(bs, 0xc0+byte(size))
	}

	byteHeader := convertBigEndian(uint(size))
	bs = append(bs, 0xf7+byte(len(byteHeader)))

	return append(bs, byteHeader...)
}

func encodeByteHeader(bs []byte, size int) []byte {
	if size < 56 {
		return append(bs, 0x80+byte(size))
	}

	byteHeader := convertBigEndian(uint(size))
	bs = append(bs, 0xb7+byte(len(byteHeader)))
	return append(bs, byteHeader...)
}

var data = make([]byte, 8)

func convertBigEndian(num uint) []byte {
	var i int
	for i = 7; num >= 1; i, num = i-1, num >> 8 {
		data[i] = byte(num)
	}

	return data[i+1:]
}

func isUint(k reflect.Kind) bool {
	return k >= reflect.Uint && k <= reflect.Uintptr
}

func isByte(typ reflect.Type) bool {
	return typ.Kind() == reflect.Uint8 && !typ.Implements(encoderInterface)
}
