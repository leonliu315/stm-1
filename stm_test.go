package stm

import (
	"sync"
	"testing"
	"time"
)

func TestDecrement(t *testing.T) {
	x := NewVar(1000)
	for i := 0; i < 500; i++ {
		go Atomically(func(tx *Tx) {
			cur := tx.Get(x).(int)
			tx.Set(x, cur-1)
		})
	}
	done := make(chan struct{})
	go func() {
		for {
			if AtomicGet(x).(int) == 500 {
				break
			}
		}
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("decrement did not complete in time")
	}
}

// read-only transaction aren't exempt from calling tx.verify
func TestReadVerify(t *testing.T) {
	read := make(chan struct{})
	x, y := NewVar(1), NewVar(2)

	// spawn a transaction that writes to x
	go func() {
		Atomically(func(tx *Tx) {
			<-read
			tx.Set(x, 3)
		})
		read <- struct{}{}
		// other tx should retry, so we need to read/send again
		read <- <-read
	}()

	// spawn a transaction that reads x, then y. The other tx will modify x in
	// between the reads, causing this tx to retry.
	var x2, y2 int
	Atomically(func(tx *Tx) {
		x2 = tx.Get(x).(int)
		read <- struct{}{}
		<-read // wait for other tx to complete
		y2 = tx.Get(y).(int)
	})
	if x2 == 1 && y2 == 2 {
		t.Fatal("read was not verified")
	}
}

func BenchmarkAtomicGet(b *testing.B) {
	x := NewVar(0)
	for i := 0; i < b.N; i++ {
		AtomicGet(x)
	}
}

func BenchmarkAtomicSet(b *testing.B) {
	x := NewVar(0)
	for i := 0; i < b.N; i++ {
		AtomicSet(x, 0)
	}
}

func BenchmarkIncrementSTM(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// spawn 1000 goroutines that each increment x by 1
		x := NewVar(0)
		for i := 0; i < 1000; i++ {
			go Atomically(func(tx *Tx) {
				cur := tx.Get(x).(int)
				tx.Set(x, cur+1)
			})
		}
		// wait for x to reach 1000
		for AtomicGet(x).(int) != 1000 {
		}
	}
}

func BenchmarkIncrementMutex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var mu sync.Mutex
		x := 0
		for i := 0; i < 1000; i++ {
			go func() {
				mu.Lock()
				x++
				mu.Unlock()
			}()
		}
		for {
			mu.Lock()
			read := x
			mu.Unlock()
			if read == 1000 {
				break
			}
		}
	}
}

func BenchmarkIncrementRWMutex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var mu sync.RWMutex
		x := 0
		for i := 0; i < 1000; i++ {
			go func() {
				mu.Lock()
				x++
				mu.Unlock()
			}()
		}
		for {
			mu.RLock()
			read := x
			mu.RUnlock()
			if read == 1000 {
				break
			}
		}
	}
}

func BenchmarkReadVarSTM(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1000)
		x := NewVar(0)
		for i := 0; i < 1000; i++ {
			go func() {
				AtomicGet(x)
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkReadVarMutex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var mu sync.Mutex
		var wg sync.WaitGroup
		wg.Add(1000)
		x := 0
		for i := 0; i < 1000; i++ {
			go func() {
				mu.Lock()
				_ = x
				mu.Unlock()
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkReadVarRWMutex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var mu sync.RWMutex
		var wg sync.WaitGroup
		wg.Add(1000)
		x := 0
		for i := 0; i < 1000; i++ {
			go func() {
				mu.RLock()
				_ = x
				mu.RUnlock()
				wg.Done()
			}()
		}
		wg.Wait()
	}
}
