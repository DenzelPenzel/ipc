package ipc

//
//func TestIPC(t *testing.T) {
//	done := make(chan struct{})
//	sm := NewShm()
//	go func() {
//		key, err := Ftok("/test", 5)
//		semid, err := sm.Shmget(key, 1, IPC_RW)
//
//	}()
//
//	<-done
//}
