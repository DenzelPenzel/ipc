package ipc

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
)

type Flock struct {
	path string
	file *os.File
}

func NewFlock(path string) (*Flock, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &Flock{
		path: path,
		file: f,
	}, nil
}

func (f *Flock) lock(exclusive bool, rest ...bool) error {
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	if len(rest) > 0 && rest[0] {
		how |= syscall.LOCK_NB
	}
	err := syscall.Flock(int(f.file.Fd()), how)
	if err != nil {
		return fmt.Errorf("can't flock path: %s, err: %s", f.path, err)
	}

	return nil
}

func (f *Flock) ExclusiveLock(rest ...bool) error {
	return f.lock(true, rest...)
}

func (f *Flock) ShareLock(rest ...bool) error {
	return f.lock(false, rest...)
}

// Close ... free all locks and closes the file
func (f *Flock) Close() error {
	return f.file.Close()
}

func (f *Flock) UnlockAll() error {
	return syscall.Flock(int(f.file.Fd()), syscall.LOCK_UN)
}

func (f *Flock) FlockMutex() IPСLock {
	return &FlockMutex{file: f, local: sync.RWMutex{}}
}

// FlockMutex ... inter-process lock by flock
type FlockMutex struct {
	file  *Flock
	local sync.RWMutex
	count int32
}

func (f *FlockMutex) RLock() {
	f.local.RLock()
	atomic.AddInt32(&f.count, 1)
	_ = f.file.ShareLock()
}

func (f *FlockMutex) RUnlock() {
	cnt := atomic.AddInt32(&f.count, -1)
	if cnt == 0 {
		_ = f.file.UnlockAll()
	}
	f.local.RUnlock()
}

func (f *FlockMutex) Lock() {
	f.local.Lock()
	_ = f.file.ExclusiveLock()
}

func (f *FlockMutex) Unlock() {
	_ = f.file.UnlockAll()
	f.local.Unlock()
}

func (f *FlockMutex) Close() {
	f.local.Lock()
	defer func() {
		f.local.Unlock()
		recover()
	}()
	_ = f.file.Close()
}
