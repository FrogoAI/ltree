package set

// GenericDataSet data set support any string or number as key
// It can test for in O(1) time
type GenericDataSet[K comparable] map[K]struct{}

// NewGenericDataSet returns a new GenericDataSet from the given data
func NewGenericDataSet[K comparable](input ...K) GenericDataSet[K] {
	set := GenericDataSet[K]{}
	for _, v := range input {
		set.Add(v)
	}

	return set
}

// Add inserts the given value into the set
func (set GenericDataSet[K]) Add(key K) {
	set[key] = struct{}{}
}

func (set GenericDataSet[K]) Union(u GenericDataSet[K]) {
	for _, key := range u.ToSlice() {
		set.Add(key)
	}
}

// Delete delete value
func (set GenericDataSet[K]) Delete(key K) {
	delete(set, key)
}

// Contains tests the membership of given key in the set
func (set GenericDataSet[K]) Contains(key K) (exists bool) {
	_, exists = set[key]
	return
}

// Count return count of elements
func (set GenericDataSet[K]) Count() int {
	return len(set)
}

// IsEmpty return if set is empty
func (set GenericDataSet[K]) IsEmpty() bool {
	return len(set) == 0
}

// ToSlice return slice of entities
func (set GenericDataSet[K]) ToSlice() []K {
	s := make([]K, set.Count())

	var i int

	for k := range set {
		s[i] = k
		i++
	}

	return s
}

// GenericOrderedDataSet is a set of strings or numbers
// It can test for in O(1) time
type GenericOrderedDataSet[K comparable] struct {
	data  map[K]struct{}
	order []K
}

// NewGenericOrderedDataSet returns a new GenericOrderedDataSet from the given data
func NewGenericOrderedDataSet[K comparable](input ...K) GenericOrderedDataSet[K] {
	set := GenericOrderedDataSet[K]{
		data: map[K]struct{}{},
	}
	for _, v := range input {
		set.Add(v)
	}

	return set
}

// Add inserts the given value into the set
func (set *GenericOrderedDataSet[K]) Add(key K) {
	if _, ok := set.data[key]; !ok {
		set.data[key] = struct{}{}
		set.order = append(set.order, key)
	}
}

// Delete delete value
func (set *GenericOrderedDataSet[K]) Delete(key K) {
	if _, ok := set.data[key]; ok {
		for i, key2 := range set.order {
			if key == key2 {
				set.order = append(set.order[:i], set.order[i+1:]...)
			}
		}

		delete(set.data, key)
	}
}

// Last last get lasts element
func (set *GenericOrderedDataSet[K]) Last() K {
	return set.order[len(set.order)-1]
}

// Contains tests the membership of given key in the set
func (set *GenericOrderedDataSet[K]) Contains(key K) (exists bool) {
	_, exists = set.data[key]
	return
}

// ToSlice return slice of entities
func (set *GenericOrderedDataSet[K]) ToSlice() []K {
	s := make([]K, set.Count())
	copy(s, set.order)

	return s
}

// Count return count of elements
func (set *GenericOrderedDataSet[K]) Count() int {
	var i int
	for range set.order {
		i++
	}

	return i
}
