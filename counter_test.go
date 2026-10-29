package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrentCounter(t *testing.T) {
	counter := 0
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < 1000; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()
			counter++
		}()
	}

	wg.Wait()

	if counter != 1000 {
		t.Errorf("Expected 1000, got %d", counter)
	}
}

func TestAtomicCounter(t *testing.T) {
	var counter atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			counter.Add(1)
		}()
	}

	wg.Wait()

	if counter.Load() != 1000 {
		t.Errorf("Expected 1000, got %d", counter.Load())
	}
}

func decrementStock(stock *atomic.Int64) bool {
	for {
		curVal := stock.Load()
		if curVal <= 0 {
			return false
		}

		newVal := curVal - 1
		if stock.CompareAndSwap(curVal, newVal) {
			return true
		}
	}
}

func TestStockDecrement(t *testing.T) {
	var stock atomic.Int64
	var success atomic.Int64
	var fail atomic.Int64
	var wg sync.WaitGroup

	stock.Store(1000)

	for i := 0; i < 1500; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			if decrementStock(&stock) {
				success.Add(1)
			} else {
				fail.Add(1)
			}
		}()
	}
	wg.Wait()

	if stock.Load() != 0 {
		t.Errorf("Stock expected 0, got %d", stock.Load())
	}
	if success.Load() != 1000 {
		t.Errorf("Success expected 1000, got %d", success.Load())
	}
	if fail.Load() != 500 {
		t.Errorf("Expected 500 failures, got %d", fail.Load())
	}
}

func TestProducerConsumer(t *testing.T) {
	ch := make(chan int)
	total := 0

	go func() {
		for num := range 100 {
			ch <- num + 1
		}
		close(ch)
	}()

	for i := range ch {
		total += i
	}
	if total != 5050 {
		t.Errorf("Error: expected %d, got %d", 5050, total)
	}
}

func TestBufferedChannel(t *testing.T) {
	total := 0
	ch := make(chan int, 3)
	ch <- 10
	ch <- 20
	ch <- 30
	if len(ch) != 3 || cap(ch) != 3 {
		t.Errorf("Expected len 3, cap 3, got len %d, cap %d", len(ch), cap(ch))
	}
	first := <-ch
	if first != 10 {
		t.Errorf("Expected 10, got %d", first)
	}
	ch <- 40
	close(ch)
	for num := range ch {
		total += num
	}
	if total != 90 {
		t.Errorf("Expected total 90, got %d", total)
	}
}

func TestWorkerCancellation(t *testing.T) {
	var wg sync.WaitGroup
	ch := make(chan int)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)

	go func() {
		defer wg.Done()
		i := 1
		for {
			select {
			case ch <- i:
				i++
			case <-ctx.Done():
				return
			}
		}
	}()

	for i := range 5 {
		if value := <-ch; value != i+1 {
			t.Errorf("Expected %d, got %d", i+1, value)
		}
	}
	cancel()
	wg.Wait()
}
