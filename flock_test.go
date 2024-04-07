package ipc

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFlockMutex_Exclusive(t *testing.T) {
	f, err := NewFlock("test")
	require.NoError(t, err)

	err = f.ShareLock(true)
	require.NoError(t, err)

	err = f.ShareLock(true)
	require.NoError(t, err)

	err = f.UnlockAll()
	require.NoError(t, err)

	err = f.ExclusiveLock()
	require.NoError(t, err)

	// use the same resource
	f2, err := NewFlock("test")
	require.NoError(t, err)

	err = f2.ExclusiveLock(true)
	require.Errorf(t, err, "can't flock: path: test, err: resource temporarily unavailable")

	err = f2.ShareLock(true)
	require.Errorf(t, err, "can't flock: path: test, err: resource temporarily unavailable")
}

func TestFlockMutex_Lock(t *testing.T) {
	f, err := NewFlock("test")
	require.NoError(t, err)
	fm := f.FlockMutex()
	fm.RLock()
	fm.RLock()
	fm.RUnlock()
	fm.RUnlock()
	fm.Lock()
	fm.Unlock()
}
