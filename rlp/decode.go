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

	dec, err := b.getDecoder(val1.Type())
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

func (buf *buffer) getDecoder(typ reflect.Type) (decoder, error) {
	kind := typ.Kind()
	switch {
	case kind == reflect.String:
		return (*buffer).decodeString, nil
	case kind == reflect.Slice || kind == reflect.Array:
	case kind == reflect.Struct:
	case kind == reflect.Ptr:
	}

	return nil, fmt.Errorf("decoder does not support type: %v", typ)
}

func (buf *buffer) decodeString(val reflect.Value) error {
	// extract string
	bs, err := buf.getBytes()
	if err != nil {
		return err
	}

	str := string(bs)

	// set string
	val.SetString(str)

	return nil
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

		bytes = dat[1 : 1+siz]
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
		buf.idx += 1 + int(headerSiz + siz)
	} else {
		return nil, fmt.Errorf("invalid leading byte: %x", dat[0])
	}

	return bytes, nil
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
