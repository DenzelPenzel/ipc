package ipc

import (
	"github.com/denzelpenzel/ipc/semlock"
	"sync"
	"syscall"
)

type LockType int8

type Lock interface {
	sync.Locker
	RLock()
	RUnlock()
	Close()
}

const (
	SemLockMode LockType = 0
	FlockMode   LockType = 1
)

// Ftok ... Generates a key that is likely to be unique for use with System V IPC
func Ftok(path string, id uint64) (uint64, error) {
	st := &syscall.Stat_t{}
	if err := syscall.Stat(path, st); err != nil {
		return 0, err
	}
	return uint64((st.Ino & 0xffff) | uint64((st.Dev&0xff)<<16) | ((id & 0xff) << 24)), nil
}

func NewLock(path string, mode LockType, ftokIds ...uint64) (Lock, error) {
	switch mode {
	case FlockMode:
		f, err := NewFlock(path)
		if err != nil {
			return nil, err
		}
		return f.FlockMutex(), nil
	case SemLockMode:
		var id uint64
		if len(ftokIds) > 0 {
			id = ftokIds[0]
		}
		key, err := Ftok(path, id)
		if err != nil {
			return nil, err
		}
		return semlock.NewSemLock(key)
	}
	return nil, nil
}
