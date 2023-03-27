# `zeropool` is a type-safe zero-allocation sync.Pool alternative for Go

## TL;DR

```go
pool := zeropool.New(func() []byte { return nil }) // The function provided makes a zeropool.Pool of a correct type using generics.
buf := pool.Get() // This is a []byte, no need to make type-assertion, no need to de-reference.
pool.Put(buf) // This does not allocate.
```

## Why?

[Go provides](https://pkg.go.dev/sync#Pool) `sync.Pool` pool implementation that allows storing `any` values (`interface{}` values). It is great but has two major drawbacks:
- It's not type-safe, a type-assertion is needed on the elements provided by `Get()`.
- Since it stores `interface{}` values, it means that your value will escape[^1] to the heap unless you store a pointer (it would escape, but maybe just once).

The second drawback is a major one, and actually is the reason why [Staticcheck SA6002](https://staticcheck.io/docs/checks#SA6002) exists. It's not unusual, for example, to use `sync.Pool` to store allocated byte slices, in which case one would do:

```go
var pool = sync.Pool{New: func() any { return new([]byte) }}

func do() {
    buf := pool.Get().(*[]byte)
    
    *buf = somethingThatNeedsABuffer(*buf)
    pool.Put(buf[:0])
}

func somethingThatNeedsABuffer(buf []byte) buf {
	buf = append(buf, []byte("something uselesss")...)
	return buf
}
```

Not great (we have to do a type assertion), not terrible (the scope is small).

However, sometimes we would want to pass that buffer to a function that would only accept `[]byte`, and that has its own lifecycle so it would take the responsibility of putting it back to the pool:

```go
var pool = sync.Pool{New: func() any { return new([]byte) }}

func do() {
    buf := pool.Get().(*[]byte)
    go somethingThatNeedsABuffer(*buf)
}

func somethingThatNeedsABuffer(buf []byte) {
	buf = append(buf, []byte("something uselesss")...)
	pool.Put(&buf)
}
```

Note that in this case, our function `somethingThatNeedsABuffer` allocates a new pointer to that slice.

Enter `zeropool`:

```go
var pool = zeropool.New(func() []byte { return nil })

func do() {
    buf := pool.Get()
    go somethingThatNeedsABuffer(buf)
}

func somethingThatNeedsABuffer(buf []byte) {
	buf = append(buf, []byte("something uselesss")...)
	pool.Put(buf)
}
```

## How to solve "SA6002 - Storing non-pointer values in sync.Pool allocates memory"

Replace your `sync.Pool` implementation by `zeropool.Pool`, and you also get the type-safety for free.

## How does it work?

`zeropool` maintains two `sync.Pool` instances: one is used as the main pool for pointers to the stored items. 
The second pool is used to hold the pointers while the code is using the items from the pool.

## Performance

It is approximately ~2x slower than `sync.Pool` if what you are storing are pointers: it doesn't make sense to pay the price in that case.
However, if you have no option but to store elements, and you need to allocate new pointers to store into `sync.Pool`, `zeropool` becomes 2-3x faster.

[^1]: Some smaller types, like scalar values, can be stored in an interface type without allocation, but you wouldn't use a `sync.Pool` for those, right?

