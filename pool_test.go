package zeropool_test

import (
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/colega/zeropool"
)

func TestPool(t *testing.T) {
	t.Run("provides correct values", func(t *testing.T) {
		pool := zeropool.New(func() []byte { return make([]byte, 1024) })
		item1 := pool.Get()
		assertEqual(t, 1024, len(item1))

		item2 := pool.Get()
		assertEqual(t, 1024, len(item2))

		pool.Put(item1)
		pool.Put(item2)

		item1 = pool.Get()
		assertEqual(t, 1024, len(item1))

		item2 = pool.Get()
		assertEqual(t, 1024, len(item2))
	})

	t.Run("is not racy", func(t *testing.T) {
		pool := zeropool.New(func() []byte { return make([]byte, 1024) })

		const iterations = 1e6
		const concurrency = math.MaxUint8
		var counter atomic.Int64

		do := make(chan struct{}, 1e6)
		for i := 0; i < iterations; i++ {
			do <- struct{}{}
		}
		close(do)

		run := make(chan struct{})
		done := sync.WaitGroup{}
		done.Add(concurrency)
		for i := 0; i < concurrency; i++ {
			go func(worker int) {
				<-run
				for range do {
					item := pool.Get()
					item[0] = byte(worker)
					counter.Add(1) // Counts and also adds some delay to add raciness.
					if item[0] != byte(worker) {
						panic("wrong value")
					}
					pool.Put(item)
				}
				done.Done()
			}(i)
		}
		close(run)
		done.Wait()
		t.Logf("Done %d iterations", counter.Load())
	})

	t.Run("does not allocate", func(t *testing.T) {
		pool := zeropool.New(func() []byte { return make([]byte, 1024) })
		// Warm up, this will alloate one slice.
		slice := pool.Get()
		pool.Put(slice)

		allocs := testing.AllocsPerRun(1000, func() {
			slice := pool.Get()
			pool.Put(slice)
		})
		assertEqualf(t, float64(0), allocs, "Should not allocate.")
	})

	t.Run("zero value is valid", func(t *testing.T) {
		var pool zeropool.Pool[[]byte]
		slice := pool.Get()
		pool.Put(slice)

		allocs := testing.AllocsPerRun(1000, func() {
			slice := pool.Get()
			pool.Put(slice)
		})
		assertEqualf(t, float64(0), allocs, "Should not allocate.")
	})
}

func BenchmarkZeropoolPool(b *testing.B) {
	b.Run("same goroutine", func(b *testing.B) {
		pool := zeropool.New(func() []byte { return make([]byte, 1024) })

		// Warmup
		item := pool.Get()
		pool.Put(item)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			item := pool.Get()
			pool.Put(item)
		}
	})

	b.Run("different goroutines", func(b *testing.B) {
		pool := zeropool.New(func() []byte { return make([]byte, 1024) })

		ch := make(chan []byte)
		go func() {
			for item := range ch {
				pool.Put(item)
			}
		}()
		defer close(ch)

		// Warmup.
		ch <- pool.Get()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch <- pool.Get()
		}
	})

}

// BenchmarkSyncPoolValue uses sync.Pool to store values, which makes an allocation on each Put call.
func BenchmarkSyncPoolValue(b *testing.B) {
	b.Run("same goroutine", func(b *testing.B) {
		pool := sync.Pool{New: func() any {
			return make([]byte, 1024)
		}}

		// Warmup.
		item := pool.Get().([]byte)
		pool.Put(item) //nolint:staticcheck // This allocates.

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			item := pool.Get().([]byte)
			pool.Put(item) //nolint:staticcheck // This allocates.
		}
	})

	b.Run("different goroutines", func(b *testing.B) {
		pool := sync.Pool{New: func() any {
			return make([]byte, 1024)
		}}

		ch := make(chan []byte)
		go func() {
			for item := range ch {
				pool.Put(item) //nolint:staticcheck // This allocates.
			}
		}()
		defer close(ch)

		// Warmup
		ch <- pool.Get().([]byte)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch <- pool.Get().([]byte)
		}
	})
}

// BenchmarkSyncPoolNewPointer uses sync.Pool to store pointers, but it calls Put with a new pointer every time.
func BenchmarkSyncPoolNewPointer(b *testing.B) {
	b.Run("same goroutine", func(b *testing.B) {
		pool := sync.Pool{New: func() any {
			v := make([]byte, 1024)
			return &v
		}}

		// Warmup
		item := pool.Get().(*[]byte)
		pool.Put(item)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			item := pool.Get().(*[]byte)
			buf := *item
			pool.Put(&buf) //nolint:staticcheck  // New pointer.
		}
	})

	b.Run("different goroutines", func(b *testing.B) {
		pool := sync.Pool{New: func() any {
			v := make([]byte, 1024)
			return &v
		}}
		ch := make(chan []byte)
		go func() {
			for item := range ch {
				pool.Put(&item) //nolint:staticcheck  // New pointer.
			}
		}()
		defer close(ch)

		// Warmup
		ch <- *(pool.Get().(*[]byte))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch <- *(pool.Get().(*[]byte))
		}
	})
}

// BenchmarkSyncPoolPointer illustrates the optimal usage of sync.Pool, not always possible.
func BenchmarkSyncPoolPointer(b *testing.B) {
	b.Run("same goroutine", func(b *testing.B) {
		pool := sync.Pool{New: func() any {
			v := make([]byte, 1024)
			return &v
		}}

		// Warmup
		item := pool.Get().(*[]byte)
		pool.Put(item)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			item := pool.Get().(*[]byte)
			pool.Put(item)
		}
	})

	b.Run("different goroutines", func(b *testing.B) {
		pool := sync.Pool{New: func() any {
			v := make([]byte, 1024)
			return &v
		}}

		ch := make(chan *[]byte)
		go func() {
			for item := range ch {
				pool.Put(item)
			}
		}()
		defer close(ch)

		// Warmup
		ch <- pool.Get().(*[]byte)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch <- pool.Get().(*[]byte)
		}
	})
}

func assertEqual(t *testing.T, expected, got interface{}) {
	t.Helper()
	if !reflect.DeepEqual(expected, got) {
		t.Logf("Expected %v, got %v", expected, got)
		t.Fail()
	}
}

func assertEqualf(t *testing.T, expected, got interface{}, msg string, args ...any) {
	t.Helper()
	if !reflect.DeepEqual(expected, got) {
		t.Logf("Expected %v, got %v", expected, got)
		t.Errorf(msg, args...)
	}
}
