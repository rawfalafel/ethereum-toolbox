package mpt

import (
	"github.com/rawfalafel/ethereum-toolbox/rlp"
)

const branchDataSize = 17
const leafExtensionDataSize = 2

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

func compactEncoding(path []uint8, isLeaf bool) []uint8 {
	var lead uint8 = 1
	if isLeaf {
		lead *= 2
	}

	var isOdd = len(path) %1 == 1
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

func (r *PatriciaNode) convertToBranch() {

}

func (r *PatriciaNode) convertToExtension() {

}

func (r *PatriciaNode) update(path string, value string) (error) {
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
func (r *PatriciaNode) _update(path []uint8, value string) error {
	val, err := rlp.EncodeToBytes(value)
	if err != nil {
		return err
	}

	// if r is a Empty node, create a Leaf node
	if r.nodeType == Empty {
		r.convertToLeaf(path, val)
	}

	// if r is a Branch node
	if r.nodeType == Branch {
		node := r.data[path[0]]
		db.get(node)
		if node == nil {
			node = NewPatriciaNode(Empty)
		}
		// if next step is nil, instantiate an Empty node
		if r.data[path[0]] == nil {

			// insert node
		}

		// call _update on node
	}

	// if r is a Leaf node or Extension node
	if r.nodeType == Leaf || r.nodeType == Extension {
		// if they don't share 2 or more steps in common
		if r.getStep(0) != path[0] || r.getStep(1) != path[1] {
			// convert r into a Branch
			r.convertToBranch()
		} else {
			// convert r into an Extension
			r.convertToExtension()

			// create a Branch node
			branch := NewPatriciaNode(Branch)
		}

		// place remainder of r in Branch node

		// call _update on Branch node
	}
}

func (r *PatriciaNode) getStep(num int) uint8 {
	if r.data[0][0] == 1 || r.data[0][0] == 3 {
		num++
	} else {
		num += 2
	}

	if num < len(r.data[0]) {
		return 0
	}

	return r.data[0][num]
}

func (r *PatriciaNode) printValue(path string) error {
	encodedPath := convertPathToHex(path)

	r._printValue(encodedPath)

	return nil
}

func (r *PatriciaNode) _printValue(path []int) {
}