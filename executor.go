package ltree

type Executor[K comparable, V any] interface {
	Key() K
	Value() V
	IsAsync() bool
	DependsOn() []K
	Skip() bool
}
