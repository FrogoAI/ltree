package ltree

type nodeState int

const (
	stateUnvisited nodeState = iota
	stateVisiting
	stateVisited
)

type Node[K comparable, V any] struct {
	value  Executor[K, V]
	parent []*Node[K, V]
	level  int
	state  nodeState
}

func NewNode[K comparable, V any](v Executor[K, V]) *Node[K, V] {
	return &Node[K, V]{
		value: v,
	}
}

func (n *Node[K, V]) AddParent(v *Node[K, V]) {
	n.parent = append(n.parent, v)

	if n.level < v.level+1 {
		n.level = v.level + 1
	}
}

func (n *Node[K, V]) HasParent(v *Node[K, V]) bool {
	for _, ch := range n.parent {
		if ch == v {
			return true
		}
	}

	return false
}
