package ipc

import (
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestSemLock(t *testing.T) {
	semLock, err := NewSemLock(1)
	require.NoError(t, err)
	semLock.Close()

	semLock, err = NewSemLock(1)
	require.NoError(t, err)
	defer semLock.Close()

	semLock.RLock()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		semLock.RLock()
		wg.Add(1)

		go func() {
			defer wg.Done()
			t.Log("semLock.Lock() 1")
			semLock.Lock() // waiting
			t.Log("semLock.Lock() 2")
			wg.Add(1)

			go func() {
				defer wg.Done()
				semLock.Lock()
				t.Log("4: Lock")
				semLock.Unlock()
				t.Log("4: Unlock")
			}()

			time.Sleep(time.Second * 1)
			t.Log("3: Lock")
			semLock.Unlock()
			t.Log("3: Unlock")
		}()

		time.Sleep(time.Second * 2)
		t.Log("1: RLock")
		semLock.RUnlock()
		t.Log("1: RUnlock")
	}()

	time.Sleep(time.Second * 3)
	t.Log("2: RLock")
	semLock.RUnlock()
	t.Log("2: RUnlock")
	wg.Wait()
}
