package ipc

import "sync"

type IPСLock interface {
	sync.Locker
	RLock()
	RUnlock()
	Close()
}
