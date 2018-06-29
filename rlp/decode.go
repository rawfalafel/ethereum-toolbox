package rlp

import (
	"fmt"
	"reflect"
)

// Decode ...
// func Decode(r io.Reader, val interface{}) error {

// }

// DecodeBytes ...
func DecodeBytes(data []byte, v interface{}) error {
	b := newBuffer(data)

	val := reflect.ValueOf(v)
	if val.IsNil() {
		return fmt.Errorf("can not decode to a non-pointer")
	}

	val1 := val.Elem()

	dec, err := getDecoder(val1.Type())
	if err != nil {
		return err
	}

	if err := dec(b, val1); err != nil {
		return err
	}

	if b.idx != len(b.dat) {
		return fmt.Errorf("did not parse entire buffer. idx: %d, length: %d", b.idx, len(b.dat))
	}

	return nil
}

func newBuffer(data []byte) *buffer {
	return &buffer{dat: data}
}

type buffer struct {
	dat []byte
	idx int // offset
}

type decoder func(*buffer, reflect.Value) error

func getDecoder(typ reflect.Type) (decoder, error) {
	kind := typ.Kind()
	switch {
	case kind == reflect.String:
		return (*buffer).decodeString, nil
	case kind == reflect.Slice || kind == reflect.Array:
		return (*buffer).decodeList, nil
	case kind == reflect.Struct:
		return (*buffer).decodeStruct, nil
	case kind == reflect.Ptr:
		return makeDecodePtr(typ)
	}

	return nil, fmt.Errorf("decoder does not support type: %v", typ)
}

func makeDecodePtr(typ reflect.Type) (decoder, error) {
	t1 := typ.Elem()
	dec, err := getDecoder(t1)
	if err != nil {
		return nil, err
	}

	return func(buf *buffer, val reflect.Value) error {
		val.Set(reflect.New(t1))
		if err := dec(buf, val.Elem()); err != nil {
			return err
		}

		return nil
	} , nil
}

func (buf *buffer) decodeList(val reflect.Value) error {
	listDat, err := buf.getList()
	if err != nil {
		return err
	}

	listBuf := newBuffer(listDat)

	typ1 := val.Type().Elem()
	dec, err := getDecoder(typ1)
	if err != nil {
		return err
	}

	sliceLen, err := listBuf.seekNumItems()
	if err != nil {
		return err
	}

	if val.Type().Kind() == reflect.Slice {
		val.Set(reflect.MakeSlice(val.Type(), sliceLen, sliceLen))
	} else if val.Type().Kind() == reflect.Array && val.Len() != sliceLen {
		return fmt.Errorf("number of items (%d) does not match array size (%d", sliceLen, val.Len())
	}

	for i := 0; i < sliceLen; i++ {
		if err := dec(listBuf, val.Index(i)); err != nil {
			return fmt.Errorf("decoder failed for list index %d: %v", i, err)
		}
	}

	if listBuf.idx != len(listBuf.dat) {
		return fmt.Errorf("did not parse entire list buffer. idx: %d, length: %d", listBuf.idx, len(listBuf.dat))
	}

	return nil
}

func (buf *buffer) seekNumItems() (int, error) {
	if buf.idx != 0 {
		return 0, fmt.Errorf("must call seekNumItems before beginning to seek items")
	}

	if len(buf.dat) == 0 {
		return 0, nil
	}

	var getFunc getItem
	if buf.dat[0] < 0xc0 {
		getFunc = (*buffer).getBytes
	} else {
		getFunc = (*buffer).getList
	}

	numItems := 0
	for ; buf.idx < len(buf.dat); {
		if _, err := getFunc(buf); err != nil {
			return 0, fmt.Errorf("failed to seek on list index %d: %v", numItems, err)
		}

		numItems++
	}

	buf.idx = 0

	return numItems, nil
}

func (buf *buffer) decodeStruct(val reflect.Value) error {
	listDat, err := buf.getList()
	if err != nil {
		return err
	}

	listBuf := newBuffer(listDat)

	for i := 0; i < val.NumField(); i++ {
		v1 := val.Field(i)
		dec, err := getDecoder(v1.Type())
		if err != nil {
			return err
		}

		if err := dec(listBuf, v1); err != nil {
			return err
		}
	}

	if listBuf.idx != len(listBuf.dat) {
		return fmt.Errorf("did not parse entire list buffer. idx: %d, length: %d", listBuf.idx, len(listBuf.dat))
	}

	return nil
}

type getItem func(*buffer) ([]byte, error)

func (buf *buffer) getList() ([]byte, error) {
	var bytes []byte

	dat := buf.getCurrentSlice()
	numBytes := len(dat)
	if dat[0] < 0xc0 {
		return nil, fmt.Errorf("invalid leading byte: %x", dat[0])
	} else if dat[0] < 0xf7 {
		siz := int(dat[0] - 0xc0)
		if 1+siz > numBytes {
			return nil, fmt.Errorf("reached end of buffer")
		}

		bytes = dat[1:1+siz]
		buf.idx += 1+siz
	} else {
		headerSiz := uint(dat[0] - 0xf7)
		if 1+headerSiz > uint(numBytes) {
			return nil, fmt.Errorf("reached end of buffer")
		}
		
		siz := buf.decodeBigEndian(dat[1:1+headerSiz])
		if 1+headerSiz+siz > uint(numBytes) {
			return nil, fmt.Errorf("reached end of buffer")
		}

		bytes = dat[1+headerSiz:1+headerSiz+siz]
		buf.idx += 1+int(headerSiz+siz)
	}
	
	return bytes, nil
}

func (buf *buffer) getBytes() ([]byte, error) {
	var bytes []byte

	dat := buf.getCurrentSlice()
	numBytes := len(dat)
	if dat[0] < 0x80 {
		bytes = dat[:1]
		buf.idx++
	} else if dat[0] < 0xb7 {
		siz := int(dat[0] - 0x80)
		if 1+siz > numBytes {
			return nil, fmt.Errorf("reached end of buffer")
		}

		bytes = dat[1:1+siz]
		buf.idx += 1 + siz
	} else if dat[0] < 0xc0 {
		headerSiz := uint(dat[0] - 0xb7)
		if 1+headerSiz > uint(numBytes) {
			return nil, fmt.Errorf("reached end of buffer")
		}

		siz := buf.decodeBigEndian(dat[1:1+headerSiz])
		if 1+headerSiz+siz > uint(numBytes) {
			return nil, fmt.Errorf("reached end of buffer")
		}

		bytes = dat[1+headerSiz:1+headerSiz+siz]
		// TODO: Can idx be int?
		buf.idx += 1 + int(headerSiz+siz)
	} else {
		return nil, fmt.Errorf("invalid leading byte: %x", dat[0])
	}

	return bytes, nil
}

func (buf *buffer) decodeString(val reflect.Value) error {
	bs, err := buf.getBytes()
	if err != nil {
		return err
	}

	str := string(bs)

	// set string
	val.SetString(str)

	return nil
}

func (buf *buffer) decodeBigEndian(dat []byte) uint {
	var out uint

	siz := len(dat)
	for i, b := range dat {
		tmp := uint(b) << uint(8*(siz-1-i))
		out += tmp
	}

	return out
}

func (buf *buffer) getCurrentSlice() []byte {
	return buf.dat[buf.idx:]
}
