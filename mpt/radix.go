package mpt

import (
	"fmt"
	"strings"

	// "github.com/rawfalafel/ethereum-toolbox/rlp"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

const branchDataSize = 17
const leafExtensionDataSize = 2

var nodeStore = map[[32]byte][]byte{}

func getNode(hash []uint8) (*PatriciaNode, bool) {
	var digest [32]byte
	copy(digest[:], []byte(hash)[:])
	data, ok := nodeStore[digest]
	if !ok {
		return nil, false
	}

	node := new(PatriciaNode)
	rlp.DecodeBytes(data, node)
	return node, true
}

func setNode(node *PatriciaNode) ([]byte, error) {
	hash, err := rlp.EncodeToBytes(node)
	if err != nil {
		return nil, err
	}

	digest := sha3.Sum256(hash)
	nodeStore[digest] = hash
	return digest[:], nil
}

const (
	// Empty ...
	Empty uint = 0
	// Branch ...
	Branch uint = 1
	// Leaf ...
	Leaf uint = 2
	// Extension ...
	Extension uint = 3
)

// PatriciaNode ...
type PatriciaNode struct {
	Data     [][]uint8
	NodeType uint
}

// NewPatriciaNode ...
func NewPatriciaNode(nodeType uint) *PatriciaNode {
	switch nodeType {
	case Empty:
		return &PatriciaNode{nil, Empty}
	case Branch:
		return &PatriciaNode{make([][]byte, branchDataSize), Branch}
	case Leaf:
		return &PatriciaNode{make([][]byte, leafExtensionDataSize), Leaf}
	case Extension:
		return &PatriciaNode{make([][]byte, leafExtensionDataSize), Extension}
	}

	panic("invalid nodeType")
}

func convertPathToHex(path string) []uint8 {
	pathAsInt := make([]uint8, len(path)*2)
	for i := 0; i < len(path); i++ {
		pathAsInt[i*2] = uint8(path[i]) / 16
		pathAsInt[i*2+1] = uint8(path[i]) % 16
	}

	return pathAsInt
}

func convertHexToString(hex []uint8) string {
	var b = strings.Builder{}
	for i := 0; i < len(hex); i += 2 {
		byt := (hex[i] << 4) + hex[i+1]
		b.WriteByte(byte(byt))
	}

	return b.String()
}

func compactEncoding(path []uint8, isLeaf bool) []uint8 {
	var lead uint8
	if isLeaf {
		lead += 2
	}

	var isOdd = len(path)%2 == 1
	size := len(path) + 1

	if isOdd {
		lead++
	} else {
		size++
	}

	out := make([]uint8, size)
	out[0] = lead

	if !isOdd {
		out[1] = 0
		copy(out[2:], path)
	} else {
		copy(out[1:], path)
	}

	return out
}

func encodePath(path string) []byte {
	return nil
}

func (r *PatriciaNode) convertToLeaf(path []uint8, value []uint8) {
	r.NodeType = Leaf
	r.Data = make([][]uint8, leafExtensionDataSize)

	r.Data[0] = compactEncoding(path, true)
	r.Data[1] = value
}

func (r *PatriciaNode) convertToBranch() error {
	// TODO: convert r into branch, update r.data, create new node with remainder, save first step in path
	node := NewPatriciaNode(Leaf)
	idx := 0

	digest, err := setNode(node)
	if err != nil {
		return err
	}

	r.Data[idx] = digest
	return nil
}

func (r *PatriciaNode) convertToExtension(baseLength int) (*PatriciaNode, error) {
	var start = 1
	if r.Data[0][0]%2 == 0 {
		start = 2
	}

	end := start + baseLength

	branch := NewPatriciaNode(Branch)

	if end != len(r.Data[0]) {
		fork := NewPatriciaNode(r.NodeType)
		fork.Data[0] = compactEncoding(r.Data[0][end+1:], r.NodeType == Leaf)
		fork.Data[1] = r.Data[1]

		digest, err := setNode(fork)
		if err != nil {
			return nil, err
		}

		branch.Data[r.Data[0][end]] = digest
		r.Data[0] = compactEncoding(r.Data[0][start:end], false)
	} else {
		branch.Data[branchDataSize-1] = r.Data[1]
	}

	r.NodeType = Extension

	return branch, nil
}

func (r *PatriciaNode) update(path string, value string) ([]byte, error) {
	encodedPath := convertPathToHex(path)

	return r._update(encodedPath, value)
}

// <01>: 'dog'
// rootNode: [ leaf 01, 'dog' ]

// <01>: 'dog'
// <02>: 'cat'
// rootNode: [ hashA <> <> ... ]
// hashA: [ hashB hashC ... ]
// hashB: [ leaf 'dog' ]
// hashC: [ leaf 'cat' ]
//
// state transitions:
// leaf -> branch if paths share only the first step
// leaf -> extension + node if paths share more than one step
// extension -> branch if paths shares only the first step
// extension -> extension + node if paths share more than one step
// empty -> leaf
func (r *PatriciaNode) _update(path []uint8, value string) ([]byte, error) {
	val, err := rlp.EncodeToBytes(value)
	if err != nil {
		return nil, err
	}

	switch {
	case r.NodeType == Empty:
		r.convertToLeaf(path, val)
		return setNode(r)
	case r.NodeType == Branch:
		node, ok := getNode(r.Data[path[0]])
		if !ok {
			node = NewPatriciaNode(Empty)
		}

		// call _update on node
		digest, err := node._update(path[1:], value)
		if err != nil {
			return nil, err
		}

		r.Data[path[0]] = digest
		return setNode(r)
	case r.NodeType == Leaf || r.NodeType == Extension:
		baseLength := r.getBaseLength(path)

		switch {
		case baseLength == 0:
			err := r.convertToBranch()
			if err != nil {
				return nil, err
			}

			node := NewPatriciaNode(Empty)
			digest, err := node._update(path[baseLength+1:], value)
			if err != nil {
				return nil, err
			}

			r.Data[path[0]] = digest
			return setNode(r)
		case r.NodeType == Leaf || (r.NodeType == Extension && baseLength != r.getPathLength()):
			// convert r into an Extension
			branch, err := r.convertToExtension(baseLength)
			if err != nil {
				return nil, err
			}

			node := NewPatriciaNode(Empty)
			digest, err := node._update(path[baseLength+1:], value)
			if err != nil {
				return nil, err
			}

			branch.Data[path[baseLength]] = digest

			digest, err = setNode(branch)
			if err != nil {
				return nil, err
			}

			r.Data[1] = digest
			return setNode(r)
		default:
			branch, ok := getNode(r.Data[1])
			if !ok {
				return nil, fmt.Errorf("node not found: %v", r.Data[1])
			}

			digest, err := branch._update(path[baseLength:], value)
			if err != nil {
				return nil, fmt.Errorf("branch update failed: %v", err)
			}

			r.Data[1] = digest
			return setNode(r)
		}
	}

	panic("this shouldn't happen")
}

func (r *PatriciaNode) getPathLength() int {
	var i = 1
	if r.Data[0][0]%2 == 0 {
		i = 2
	}

	return len(r.Data[0]) - i
}

func (r *PatriciaNode) getBaseLength(path []uint8) int {
	var i int
	var j = 1
	if r.Data[0][0]%2 == 0 {
		j = 2
	}

	for i = 0; i < len(path) && j < len(r.Data[0]); {
		if r.Data[0][j] != path[i] {
			break
		}
		i++
		j++
	}

	return i
}

func (r *PatriciaNode) getStep(num int) uint8 {
	if r.Data[0][0]%2 == 0 {
		num++
	} else {
		num += 2
	}

	if num < len(r.Data[0]) {
		return 0
	}

	return r.Data[0][num]
}

func (r *PatriciaNode) getValue(path string) (string, error) {
	encodedPath := convertPathToHex(path)
	fmt.Printf("path: %v\n", encodedPath)
	defer fmt.Print("\n")

	return r._getValue(encodedPath)
}

func (r *PatriciaNode) _getValue(path []uint8) (string, error) {
	switch r.NodeType {
	case Extension:
		fmt.Printf("extension: %v\n", r.Data)
		node, ok := getNode(r.Data[1])
		if !ok {
			return "", fmt.Errorf("not found")
		}

		start := r.getPathLength()
		return node._getValue(path[start:])
	case Leaf:
		val := new(string)
		err := rlp.DecodeBytes(r.Data[1], val)
		if err != nil {
			return "", err
		}
		fmt.Printf("leaf: %v %v\n", r.Data[0], *val)
		return *val, nil
	case Branch:
		fmt.Printf("branch: %v\n", r.Data)
		if len(path) == 0 {
			val := new(string)
			dat := []byte(r.Data[branchDataSize-1])
			rlp.DecodeBytes(dat, val)
			return *val, nil
		}

		node, ok := getNode(r.Data[path[0]])
		if !ok {
			return "", fmt.Errorf("path not found")
		}

		return node._getValue(path[1:])
	case Empty:
		println("Empty")
		return "", fmt.Errorf("empty node")
	}

	panic("unknown type")
}
