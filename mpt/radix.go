package mpt

import (
	"fmt"
	"strings"
	"github.com/rawfalafel/ethereum-toolbox/rlp"
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

// NodeType ...
type NodeType int

const (
	// Empty ...
	Empty     NodeType = 0
	// Branch ...
	Branch    NodeType = 1
	// Leaf ...
	Leaf      NodeType = 2
	// Extension ...
	Extension NodeType = 3
)

// PatriciaNode ...
type PatriciaNode struct {
	data [][]uint8
	nodeType NodeType
}

// NewPatriciaNode ...
func NewPatriciaNode(nodeType NodeType) (*PatriciaNode) {
	switch nodeType {
	case Empty:
		return &PatriciaNode{ nil, Empty }
	case Branch:
		return &PatriciaNode{ make([][]byte, branchDataSize), Branch }
	case Leaf:
		return &PatriciaNode{ make([][]byte, leafExtensionDataSize), Leaf }
	case Extension:
		return &PatriciaNode{ make([][]byte, leafExtensionDataSize), Extension }
	}

	panic("invalid nodeType")
}

func convertPathToHex(path string) []uint8 {
	pathAsInt := make([]uint8, len(path) * 2)
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
	var lead uint8 = 0
	if isLeaf {
		lead += 2
	}

	var isOdd = len(path) % 2 == 1
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
	r.nodeType = Leaf
	r.data = make([][]uint8, leafExtensionDataSize)

	r.data[0] = compactEncoding(path, true)
	r.data[1] = value
}

func (r *PatriciaNode) convertToBranch() (error) {
	// TODO: convert r into branch, update r.data, create new node with remainder, save first step in path
	node := NewPatriciaNode(Leaf)
	idx := 0

	digest, err := setNode(node)
	if err != nil {
		return err
	}

	r.data[idx] = digest
	return nil
}

func (r *PatriciaNode) convertToExtension(baseLength int) (*PatriciaNode, error) {
	var start = 1
	if r.data[0][0] % 2 == 0 {
		start = 2
	}

	end := start+baseLength

	fork := NewPatriciaNode(r.nodeType)
	fork.data[0] = compactEncoding(r.data[0][end+1:], r.nodeType == Leaf)
	fork.data[1] = r.data[1]

	digest, err := setNode(fork)
	if err != nil {
		return nil, err
	}

	branch := NewPatriciaNode(Branch)
	branch.data[r.data[0][end]] = digest

	r.data[0] = compactEncoding(r.data[0][start:end], false)
	r.nodeType = Extension

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
	case r.nodeType == Empty:
		r.convertToLeaf(path, val)
		return setNode(r)
	case r.nodeType == Branch:
		node, ok := getNode(r.data[path[0]])
		if !ok {
			node = NewPatriciaNode(Empty)
		}

		// call _update on node
		digest, err := node._update(path[1:], value)
		if err != nil {
			return nil, err
		}

		r.data[path[0]] = digest
		return setNode(r)
	case r.nodeType == Leaf || r.nodeType == Extension:
		baseLength := r.getBaseLength(path)

		// if the first step isn't the same
		if baseLength == 0 {
			// convert r into a Branch
			err := r.convertToBranch()
			if err != nil {
				return nil, err
			}

			node := NewPatriciaNode(Empty)
			digest, err := node._update(path[baseLength+1:], value)
			if err != nil {
				return nil, err
			}

			r.data[path[0]] = digest
			return setNode(r)
		}

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

		branch.data[path[baseLength]] = digest

		println("branch type:", branch.nodeType)
		digest, err = setNode(branch)
		if err != nil {
			return nil, err
		}

		r.data[1] = digest
		return setNode(r)
	}

	panic("this shouldn't happen")
}

func (r *PatriciaNode) getPathLength() int {
	var i = 1
	if r.data[0][0] % 2 == 0 {
		i = 2
	}

	return len(r.data[0]) - i
}

func (r *PatriciaNode) getBaseLength(path []uint8) int {
	var i = 1
	if r.data[0][0] % 2 == 0 {
		i = 2
	}

	for j := 0; j < len(r.data[0]); j++ {
		if r.data[0][i] != path[i] {
			continue
		}
		i++
	}

	return i
}

func (r *PatriciaNode) getStep(num int) uint8 {
	if r.data[0][0] %2 == 0 {
		num++
	} else {
		num += 2
	}

	if num < len(r.data[0]) {
		return 0
	}

	return r.data[0][num]
}

func (r *PatriciaNode) getValue(path string) (string, error) {
	encodedPath := convertPathToHex(path)

	return r._getValue(encodedPath)
}

func (r *PatriciaNode) _getValue(path []uint8) (string, error) {
	switch r.nodeType {
	case Extension:
		println(fmt.Sprintf("extension: %v", r.data))
		node, ok := getNode(r.data[1])
		if !ok {
			return "", fmt.Errorf("not found")
		}

		start := r.getPathLength()
		return node._getValue(path[start:])
	case Leaf:
		val := new(string)
		err := rlp.DecodeBytes(r.data[1], val)
		if err != nil {
			return "", err
		}
		println(fmt.Sprintf("leaf: %v %v", r.data[0], *val))
		return *val, nil
	case Branch:
		println(fmt.Sprintf("branch: %v", r.data))
		if len(path) == 0 {
			val := new(string)
			dat := []byte(r.data[branchDataSize-1])
			rlp.DecodeBytes(dat, val)
			return *val, nil
		}

		node, ok := getNode(r.data[path[0]])
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