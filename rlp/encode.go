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
	return encode(v)
}

// Encode ...
func Encode(w io.Writer, v interface{}) error {
	bs, err := encode(v)
	if err != nil {
		return err
	}

	_, err = w.Write(bs)
	if err != nil {
		return fmt.Errorf("failed to write: %v", err)
	}

	return nil
}

func encode(v interface{}) ([]byte, error) {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	i := getInfo(typ)

	siz, err := i.s(val)
	if err != nil {
		return nil, err
	}

	bs := make([]byte, 0, siz)
	bs = i.w(val, bs)

	if len(bs) != siz {
		return nil, fmt.Errorf("Size doesn't match: %d but should be %d", len(bs), siz)
	}

	return bs, nil
}

var infoCache = map[reflect.Type]*encodeInfo{}

func getInfo(typ reflect.Type) *encodeInfo {
	ei, ok := infoCache[typ]

	if !ok {
		ei = &encodeInfo{}
		infoCache[typ] = ei
		ei.populate(typ)
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

	kind := typ.Kind()
	switch {
	case typ.Implements(encoderInterface):
		ei.s, ei.w = makeEncoderFuncs(typ)
	case kind == reflect.Interface:
		ei.s, ei.w = interfaceSizer, interfaceWriter
	case typ.AssignableTo(bigIntPtr):
		ei.s, ei.w = bigIntPtrSizer, bigIntPtrWriter
	case typ.AssignableTo(bigInt):
		ei.s, ei.w = bigIntNoPtrSizer, bigIntNoPtrWriter
	case isUint(kind):
		ei.s, ei.w = uintSizer, uintWriter
	case kind == reflect.String:
		ei.s, ei.w = stringSizer, stringWriter
	case kind == reflect.Bool:
		ei.s, ei.w = boolSizer, boolWriter
	case kind == reflect.Slice && isByte(typ.Elem()):
		ei.s, ei.w = byteSliceSizer, byteSliceWriter
	case kind == reflect.Array && isByte(typ.Elem()):
		ei.s, ei.w = byteArraySizer, byteArrayWriter
	case kind == reflect.Slice || kind == reflect.Array: 
		ei.s, ei.w = makeSliceFuncs(typ)
	case kind == reflect.Struct:
		s, w, _ := makeStructFuncs(typ)
		ei.s, ei.w = s, w
	case kind == reflect.Ptr:
		ei.s, ei.w = makePtrFuncs(typ)
	}
}

func interfaceSizer(v reflect.Value) (int, error) {
	if v.IsNil() {
		return 1, nil
	}

	v1 := v.Elem()
	info := getInfo(v1.Type())
	return info.s(v1)
}

func interfaceWriter(v reflect.Value, b []byte) []byte {
	if v.IsNil() {
		return append(b, 0xc0)
	}

	v1 := v.Elem()
	info := getInfo(v1.Type())
	return info.w(v1, b)
}

func makePtrFuncs(typ reflect.Type) (sizer, writer) {
	ei := getInfo(typ.Elem())

	return func(v reflect.Value) (int, error) {
		if v.IsNil() { 
			return 1, nil
		}

		return ei.s(v.Elem())		
	}, func(v reflect.Value, b []byte) []byte {
		if v.IsNil() {
			t1 := typ.Elem()
			k1 := t1.Kind()

			if k1 == reflect.Array && isByte(t1.Elem()) {
				return append(b, 0x80)
			} else if k1 == reflect.Struct || k1 == reflect.Array {
				return append(b, 0xc0)
			} 

			v1 := reflect.Zero(t1)	
			return getInfo(t1).w(v1, b)
		}

		return ei.w(v.Elem(), b)
	} 
}

func bigIntNoPtrSizer(v reflect.Value) (int, error) {
	i := v.Interface().(big.Int)
	return bigIntSizer(&i)
}

func bigIntPtrSizer(v reflect.Value) (int, error) {
	if v.IsNil() {
		return 1, nil
	} 

	v1 := v.Interface().(*big.Int)
	return bigIntSizer(v1)
}

func bigIntSizer(i *big.Int) (int, error) {
	sign := i.Sign()
	if sign == -1 {
		return 0, fmt.Errorf("rlp: cannot encode negative *big.Int")
	} else if sign == 0 {
		return 1, nil
	} 

	intAsBytes := i.Bytes()
	byteHeaderSize, err := getByteHeaderSize(intAsBytes)
	if err != nil {
		return 0, err
	}

	return byteHeaderSize + len(intAsBytes), nil
}

func bigIntNoPtrWriter(v reflect.Value, b []byte) []byte {
	i := v.Interface().(big.Int)
	return bigIntWriter(&i, b)
}

func bigIntPtrWriter(v reflect.Value, b []byte) []byte {
	if v.IsNil() {
		return append(b, 0x80)
	}

	v1 := v.Interface().(*big.Int) 
	return bigIntWriter(v1, b)
}

func bigIntWriter(i *big.Int, b []byte) []byte {
	vb := i.Bytes()
	if len(vb) == 1 {
		return encodeByte(b, vb[0])
	} 
		
	return encodeBytes(b, vb)
}

func nilSizer(_ reflect.Value) (int, error) {
	return 1, nil
}

func nilWriter(_ reflect.Value, b []byte) []byte {
	return append(b, 0xc0)
}

func makeEncoderFuncs(typ reflect.Type) (sizer, writer) {
	dataCache := map[reflect.Value][]byte{}
	return func(v reflect.Value) (int, error) {
		wz := newWriter(0)
		// Can this be the pointer to avoid unnecessary copy?
		if err := v.Interface().(Encoder).EncodeRLP(wz); err != nil {
			return 0, err
		}

		dataCache[v] = wz.data

		return len(wz.data), nil
	}, func(v reflect.Value, b []byte) []byte {
		return append(b, dataCache[v]...)
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

func boolSizer(reflect.Value) (int, error) {
	return 1, nil
}

func boolWriter(v reflect.Value, b []byte) []byte {
	val := v.Bool()
	if val {
		return append(b, 0x01)
	}

	return append(b, 0x80)
}

func makeStructFuncs(typ reflect.Type) (sizer, writer, error) {
	fs, err := getFieldInfo(typ)
	if err != nil {
		return nil, nil, err
	}

	sizer := func(v reflect.Value) (int, error) {
		siz := 0
		for i := 0; i < len(fs); i++ {
			f := v.Field(fs[i].idx)

			fsiz, err := fs[i].ei.s(f)
			if err != nil {
				return 0, fmt.Errorf("error with %v: %v", fs[i].name, err)
			}

			siz += fsiz
		}

		headerSize, err := getListHeaderSize(siz)
		if err != nil {
			return 0, err
		}

		return siz + headerSize, nil
	}

	return sizer, func(v reflect.Value, b []byte) []byte {
		siz, _ := sizer(v)

		b = encodeListHeader(b, deriveListHeaderSize(siz))
		for i := 0; i < len(fs); i++ {
			f := v.Field(fs[i].idx)

			b = fs[i].ei.w(f, b)
		}

		return b
	},
	nil
}

func makeSliceFuncs(typ reflect.Type) (sizer, writer) {
	elemInfo := getInfo(typ.Elem())

	sizer := func(v reflect.Value) (int, error) {
		siz := 0
		for i:= 0; i < v.Len(); i++ {
			v0 := v.Index(i)
			siz0, err := elemInfo.s(v0)
			if err != nil {
				return 0, fmt.Errorf("failed to fetch size for index %d: %v", i, err)
			}

			siz += siz0
		}

		listHeaderSize, err := getListHeaderSize(siz)
		if err != nil {
			return 0, fmt.Errorf("failed to calculate list header size: %v", err)
		}

		return siz + listHeaderSize, nil
	}

	return sizer, func(v reflect.Value, b []byte) []byte {
		siz, _ := sizer(v)

		b = encodeListHeader(b, deriveListHeaderSize(siz))
		for i := 0; i < v.Len(); i++ {
			v0 := v.Index(i)
			b = elemInfo.w(v0, b)
		}

		return b
	}
}

func deriveListHeaderSize(siz int) int {
	switch {
	case siz < 56:
		return siz - 1 
	case siz < (1 << 8) + 2: 
		return siz - 2 
	case siz < (1 << 16) + 3:
		return siz - 3
	case siz < (1 << 24) + 4:
		return siz - 4
	case siz < (1 << 32) + 5:
		return siz - 5
	case siz < (1 << 40) + 6:
		return siz - 6
	case siz < (1 << 48) + 7:
		return siz - 7
	case siz < (1 << 56) + 8:
		return siz - 8
	}

	panic("this shouldn't happen")
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
	idx      int
	ei       *encodeInfo
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
		f := &fieldInfo{name: structF.Name, idx: i, ei: ei}
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


func getListHeaderSize(size int) (int, error) {
	if size < 56 {
		return 1, nil
	} else if encodedSize := getBigEndianSize(uint(size)); encodedSize > 9 {
		return 0, fmt.Errorf("encoding size exceeded limit: %d bytes long", size)
	} else {
		return encodedSize + 1, nil
	}
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

func byteArraySizer(v reflect.Value) (int, error) {
	len := v.Len()
	if len == 0 && v.Index(0).Interface().(byte) <= 0x7f {
		return 1, nil
	} else if len < 56 {
		return 1+len, nil
	} else if encodedSize := getBigEndianSize(uint(len)); encodedSize > 9 {
		return 0, fmt.Errorf("encoding size limit exceeded: %d bytes long", len)
	} else {
		return 1+encodedSize+len, nil
	}
}

func byteArrayWriter(v reflect.Value, b []byte) []byte {
	if !v.CanAddr() {
		// Slice requires the value to be addressable.
		// Make it addressable by copying.
		copy := reflect.New(v.Type()).Elem()
		copy.Set(v)
		v = copy
	}
	size := v.Len()
	slice := v.Slice(0, size).Bytes()
	return encodeBytes(b, slice)
}

func byteSliceSizer(v reflect.Value) (int, error) {
	// TODO: Calculate without converting to bytes
	bytes := v.Bytes()

	size, err := getByteHeaderSize(bytes)
	if err != nil {
		return 0, err
	}

	return size + len(bytes), nil
}

func byteSliceWriter(v reflect.Value, b []byte) []byte {
	bytes := v.Bytes()

	if len(bytes) == 1 {
		return encodeByte(b, bytes[0])
	}

	return encodeBytes(b, bytes)
}

func uintSizer(v reflect.Value) (int, error) {
	v1 := v.Uint()

	if v1 < 128 {
		return 1, nil
	}

	return getBigEndianSize(uint(v1)) + 1, nil
}

func uintWriter(v reflect.Value, b []byte) []byte {
	v1 := v.Uint()
	if v1 == 0 {
		return append(b, 0x80)
	} else if v1 < 128 {
		return encodeByte(b, byte(v1))
	} 

	return encodeBytes(b, convertBigEndian(uint(v1)))
}

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
