package main

func main() {
	// Implement a radix trie
	// Each node consists of an array of length 16 and a value
	// The items in the array represent the possible values (0-E)
	// The possible values are NULL or a pointer to another node
	radixTrie := NewPatriciaNode(Empty)

	radixTrie.update("dog", "rare")
	radixTrie.printValue("dog")
}

const branchDataSize = 17
const leafExtensionDataSize = 2

type NodeType int

const (
	Empty     NodeType = 0
	Branch    NodeType = 1
	Leaf      NodeType = 2
	Extension NodeType = 3
)

type PatriciaNode struct {
	data []byte
	nodeType NodeType
}

func NewPatriciaNode(nodeType NodeType) (*PatriciaNode) {
	switch nodeType {
	case Empty:
		return &PatriciaNode{ nil, Empty}
	case Branch:
		return &PatriciaNode{ make([]byte, branchDataSize), Branch }
	case Leaf:
		return &PatriciaNode{ make([]byte, leafExtensionDataSize), Leaf }
	case Extension:
		return &PatriciaNode{ make([]byte, leafExtensionDataSize), Extension }
	}
}

func convertPathToHex(path string) []int {
	pathAsInt := make([]int, len(path) * 2)
	for i := 0; i < len(path); i++ {
		pathAsInt[i] = int(path[i]) / 16
		pathAsInt[i + 1] = int(path[i]) % 16
	}

	return pathAsInt
}

func encodePath(path string) []byte {

}

func (r *PatriciaNode) convertToLeaf() {
	r.nodeType = Leaf
	r.data = make([]byte, leafExtensionDataSize)
}

func (r *PatriciaNode) update(path string, value string) (error) {
	encodedPath := convertPathToHex(path)

	r._update(encodedPath, value)

	return nil
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
func (r *PatriciaNode) _update(path []int, value string) {
	// If r is a Empty node, create a Leaf node
	if r.nodeType == Empty {
		r.convertToLeaf()
		// set data field
	}

	// If r is a Branch node
	if r.nodeType == Branch {
		// if next step is nil, instantiate an Empty node
		if r.data[path[0]] == 0 {

			// insert node

			//
		}

		// call _update on that node
	}

	// If r is a Leaf node or Extension node
	if r.nodeType == Leaf || r.nodeType == Extension {
		// If they only share the first step in common
		if int(r.data[0]) == path[0] && int(r.data[1]) != path[1] {
			// convert r into a Branch

		} else {
			// convert r into an Extension

			// create a Branch node
		}

		// place remainder of r in Branch node

		// call _update on Branch node
	}
}

func (r *PatriciaNode) printValue(path string) error {
	encodedPath := convertPathToHex(path)

	r._printValue(encodedPath)

	return nil
}

func (r *PatriciaNode) _printValue(path []int) {
}
