package ipc

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// semlock using process semaphores to implement read-write locks

const (
	/* ipcs ctl cmds */
	SEM_STAT     = 18
	SEM_INFO     = 19
	SEM_STAT_ANY = 20
	SEM_UNDO     = 0x1000 // undo the operation on exit

	/* Commands for `semctl'.  */
	GETPID  = 11 /* get sempid */
	GETALL  = 13 /* get all semval's */
	GETNCNT = 14 /* get semncnt */
	GETZCNT = 15 /* get semzcnt */
	SETVAL  = 16 /* set semval */
	SETALL  = 17 /* set all semval's */
)

var (
	hmsRl = []SemOp{{
		SemNum:  0,
		SemOp:   0,
		SemFlag: SEM_UNDO,
	}, {
		SemNum:  1,
		SemOp:   1,
		SemFlag: SEM_UNDO,
	}}
	hmsRUl = []SemOp{{
		SemNum:  1,
		SemOp:   -1,
		SemFlag: SEM_UNDO,
	}}
	hmsWl = []SemOp{
		{
			SemNum:  1,
			SemOp:   0,
			SemFlag: SEM_UNDO,
		},
		{
			SemNum:  0,
			SemOp:   0,
			SemFlag: SEM_UNDO,
		},
		{
			SemNum:  0,
			SemOp:   1,
			SemFlag: SEM_UNDO,
		},
	}
	hmsWUl = []SemOp{{
		SemNum:  0,
		SemOp:   -1,
		SemFlag: SEM_UNDO,
	}}
)

type SemOp struct {
	SemNum  uint16
	SemOp   int16
	SemFlag int16
}

// Semget ... GetMsg a semaphore set identifier or create a semaphore set object and return the semaphore set identifier
// key:
//   - 0(IPC_PRIVATE): A new semaphore set object will be created
//   - A 32-bit integer greater than 0: The operation is determined by the parameter semflag.
//     This value is usually required to come from the IPC key value returned by Ftok.
//
// nsems:
// - number of semaphores in the created semaphore set. This parameter is only valid when creating a semaphore set.
//
// semflag:
// - 0: GetMsg the semaphore set identifier. If it does not exist, the function will report an error.
//
//   - IPC_CREAT: When semflag & IPC_CREAT is true, if there is no semaphore set with a key value equal to key in the kernel,
//     a new semaphore set will be created; if such a semaphore set exists, the identifier of this semaphore set will be returned.
//
//   - IPC_CREAT|IPC_EXCL: If there is no semaphore set with a key value equal to key in the kernel,
//     create a new message queue; if such a semaphore set exists, an error will be reported
//
// Notes:
//
//	The above semflag parameter is a mode SemFlag parameter.
//	When used, it needs to be calculated with the IPC object access permission (such as 0600)
//	to determine the access permission of the semaphore set.
//	If successful, the return value will be the semaphore set identifier,
//	otherwise, -1 is returned, with errno indicating the error.
func Semget(key uint64, nsems int, semflag int) (int, error) {
	id, _, err := syscall.Syscall(syscall.SYS_SEMGET, uintptr(key), uintptr(nsems), uintptr(semflag))
	semid := int(id)
	if semid < 0 {
		return semid, err
	}
	return semid, nil
}

// Semop ... executes operations on specific semaphores within the semaphore set identified by semid.
// Each element in the array pointed to by sops specifies an operation to be performed on a single semaphore.
// These elements are of type struct sembuf, which contains the following struct:
//
//	struct sembuf {
//	    short sem_num; /* semaphore number in the semaphore collection, 0 represents the first semaphore*/
//	    short sem_op; /* semaphore operation */
//	    short sem_flg; /* operation flags */
//	}
//
// Flags recognized in sem_flg are IPC_NOWAIT and SEM_UNDO.
// IPC_NOWAIT sets the semaphore operation not to wait
// SEM_UNDO option will cause the kernel to record an UNDO record related to the calling process.
// If the process crashes, the count value of the corresponding semaphore will be automatically
// restored based on the UNDO record of this process.
func Semop(semid int, sops []SemOp) (bool, error) {
	id, _, err := syscall.Syscall(syscall.SYS_SEMOP, uintptr(semid), uintptr(unsafe.Pointer(&sops[0])), uintptr(len(sops)))
	var ok = true
	if int(id) < 0 {
		ok = false
	}
	if err != 0 && !errors.Is(err, syscall.EAGAIN) {
		return ok, err
	}
	return ok, nil
}

// Semctl ... Removes the semaphore set with the given id
func Semctl(semid int, cmd int) error {
	id, _, err := syscall.Syscall(syscall.SYS_SEMCTL, uintptr(semid), IPC_RMID, uintptr(cmd))
	if int(id) < 0 {
		return err
	}
	return nil
}

type SemLock struct {
	id    int
	local sync.RWMutex
}

func NewSemLock(id uint64) (*SemLock, error) {
	semid, err := Semget(id, 2, IPC_CREAT|IPC_EXCL|1023)
	if errors.Is(err, syscall.EEXIST) {
		semid, err = Semget(id, 2, 0)
	}
	if err != nil {
		return nil, err
	}
	return &SemLock{id: semid}, nil
}

func (s *SemLock) Lock() {
	for {
		_, err := Semop(s.id, hmsWl)
		switch {
		case err == nil:
			return
		case errors.Is(err, syscall.EINTR):
		case errors.Is(err, syscall.EINVAL):
			s.local.Lock()
			return
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func (s *SemLock) Unlock() {
	ok, err := Semop(s.id, hmsWUl)
	if !ok || errors.Is(err, syscall.EINVAL) {
		fmt.Fprintf(os.Stderr, "error unlock sem: error(%v)", err)
		s.local.Unlock()
	}
}

func (s *SemLock) RLock() {
	for {
		_, err := Semop(s.id, hmsRl)
		switch {
		case err == nil:
			return
		case errors.Is(err, syscall.EINTR):
		case errors.Is(err, syscall.EINVAL):
			s.local.RLock()
			return
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func (s *SemLock) RUnlock() {
	ok, err := Semop(s.id, hmsRUl)
	if !ok || errors.Is(err, syscall.EINVAL) {
		s.local.RUnlock()
	}
}

func (s *SemLock) Close() {
	err := Semctl(s.id, IPC_RMID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error closing sem: error(%v)", err)
	}
}
