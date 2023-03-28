package zeropool

import "sync"

// Pool is a type-safe pool of items that does not allocate pointers to items.
// That is not entirely true, it does allocate sometimes, but not most of the time,
// just like the usual sync.Pool pools items most of the time, except when they're evicted.
// It does that by storing the allocated pointers in a secondary pool instead of letting them go,
// so they can be used later to store the items again.
//
// Zero value of Pool[T] is valid, and it will return zero values of T if nothing is pooled.
type Pool[T any] struct {
	// items holds pointers to the pooled items, which are valid to be used.
	items sync.Pool
	// pointers holds just pointers to the pooled item types.
	// The values referenced by pointers are not valid to be used (as they're used by some other caller)
	// and it is safe to overwrite these pointers.
	pointers sync.Pool
}

// New creates a new Pool[T] with the given function to create new items.
// A Pool must not be copied after first use.
func New[T any](item func() T) Pool[T] {
	return Pool[T]{
		items: sync.Pool{
			New: func() interface{} {
				val := item()
				return &val
			},
		},
	}
}

// Get returns an item from the pool, creating a new one if necessary.
// Get may be called concurrently from multiple goroutines.
func (p *Pool[T]) Get() T {
	pooled := p.items.Get()
	if pooled == nil {
		// The only way this can happen is when someone is using the zero-value of zeropool.Pool, and items pool is empty.
		// We don't have a pointer to store in p.pointers, so just return the empty value.
		var zero T
		return zero
	}

	ptr := pooled.(*T)
	item := *ptr // ptr still holds a reference to a copy of item, but nobody will use it.
	p.pointers.Put(ptr)
	return item
}

// Put adds an item to the pool.
func (p *Pool[T]) Put(item T) {
	var ptr *T
	if pooled := p.pointers.Get(); pooled != nil {
		ptr = pooled.(*T)
	} else {
		ptr = new(T)
	}
	*ptr = item
	p.items.Put(ptr)
}
