package ltree

type Entry[K comparable, V any] struct {
	key     K
	value   V
	need    []K  // related nodes
	isAsync bool // does entity contain blocked operation
	skip    func() bool
}

func (e *Entry[K, V]) Key() K {
	return e.key
}

func (e *Entry[K, V]) Value() V {
	return e.value
}

func (e *Entry[K, V]) DependsOn() []K {
	return e.need
}

func (e *Entry[K, V]) IsAsync() bool {
	return e.isAsync
}

func (e *Entry[K, V]) Skip() bool {
	if e.skip == nil {
		return false
	}

	return e.skip()
}

func NewEntry[K comparable, V any](key K, value V, need []K, isAsync bool, skip func() bool) *Entry[K, V] {
	return &Entry[K, V]{key: key, value: value, need: need, isAsync: isAsync, skip: skip}
}
