package zeropool_test

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/colega/zeropool"
)

func TestPool(t *testing.T) {
	t.Run("provides correct values", func(t *testing.T) {
		pool := zeropool.New(func() []byte { return make([]byte, 1024) })
		item1 := pool.Get()
		require.Equal(t, 1024, len(item1))

		item2 := pool.Get()
		require.Equal(t, 1024, len(item2))

		pool.Put(item1)
		pool.Put(item2)

		item1 = pool.Get()
		require.Equal(t, 1024, len(item1))

		item2 = pool.Get()
		require.Equal(t, 1024, len(item2))
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
		require.Equal(t, float64(0), allocs, "Should not allocate.")
	})
}
