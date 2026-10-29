package main

import (
	"slices"
	"sync"
	"testing"
)

func TestWorkerPool(t *testing.T) {
	const numJobs = 10
	const numWorkers = 3
	expectedValues := []int{1, 4, 9, 16, 25, 36, 49, 64, 81, 100}

	jobs := make(chan int, numJobs)
	results := make(chan int, numJobs)
	var resultValues []int

	var wg sync.WaitGroup

	for range numWorkers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for job := range jobs {
				results <- job * job
			}
		}()
	}

	for i := range numJobs {
		jobs <- i + 1
	}

	close(jobs)
	wg.Wait()
	close(results)

	for res := range results {
		resultValues = append(resultValues, res)
	}

	slices.Sort(resultValues)
	if !slices.Equal(expectedValues, resultValues) {
		t.Errorf("Expected %v, got %v", expectedValues, resultValues)
	}
}
