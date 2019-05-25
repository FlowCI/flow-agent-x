package util

import (
	"sync"
	"time"
)

func Wait(group *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})

	go func() {
		group.Wait()
		close(c)
	}()

	select {
	case <- c:
		return true
	case <-time.After(timeout):
		defer close(c)
		return false
	}
}