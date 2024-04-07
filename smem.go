package ipc

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Implementation of the shared memory functions

const (
	/* Commands for `shmctl'.  */
	SHM_LOCK   = 11 // lock segment (root only)
	SHM_UNLOCK = 12 // unlock segment (root only)

	/* shm ctl commands */
	SHM_STAT     = 13
	SHM_INFO     = 14
	SHM_STAT_ANY = 15

	/* Flags for `shmat'.  */
	SHM_RDONLY = 010000  // attach read-only else read-write
	SHM_RND    = 020000  // round attach address to SHMLBA
	SHM_REMAP  = 040000  // take-over region on attach
	SHM_EXEC   = 0100000 // execution access

	/* shm_mode upper byte flags */
	SHM_DEST      = 01000  // segment will be destroyed on last detach
	SHM_LOCKED    = 02000  // segment will not be swapped
	SHM_HUGETLB   = 04000  // segment is mapped via hugetlb
	SHM_NORESERVE = 010000 // don't check for reservations

	IPC_RMID = 0 // remove command
)

type shminfo struct {
	sync.RWMutex
	id2Size   map[int]uint64            // {id -> size}
	addr2Size map[unsafe.Pointer]uint64 // {addr -> size}
	addr2Id   map[unsafe.Pointer]int    // { addr -> id }
}

// Shmget ... Get a shared memory identifier or create a shared memory object
// key: A 32-bit integer greater than 0: The operation is governed by the parameter shmflg,
// typically derived from the IPC key value returned by Ftok
// size: An integer greater than 0: representing the size of the new shared memory, measured in bytes
// shmflg:
//   - 0: obtain the shared memory identifier. If it does not exist, the function will generate an error
//   - IPC_CREAT: When the condition shmflg & IPC_CREAT is true, a new shared memory segment will be created
//     if no shared memory with a key value equal to 'key' exists in the kernel. If such shared memory already exists,
//     its identifier will be returned
//   - IPC_CREAT|IPC_EXCL: Creates a new shared memory segment if no segment
//     with a key value equal to 'key' exists in the kernel; otherwise, an error is reported.
//
// The shmflg parameter above represents a mode flag used in shared memory operations.
// When utilized, it should be computed alongside the IPC object access permissions (e.g., 0600)
// to ascertain the access permissions for the shared memory segment
func (s *shminfo) Shmget(key, size uint64, shmflg int) (int, error) {
	_sid, _, err := syscall.Syscall(syscall.SYS_SHMGET, uintptr(key), uintptr(size), uintptr(shmflg))
	if err != 0 {
		return 0, err
	}
	sid := int(_sid)
	s.Lock()
	s.id2Size[sid] = size
	s.Unlock()
	return sid, nil
}

// Shmat ...Maps shared memory area object to the address space of the calling process
// After a fork, the child process inherits the connected shared memory address.
// However, after an exec, the child process is automatically detached from the connected shared memory address.
// Furthermore, when the process terminates, the connected shared memory address
// will be automatically detached (or 'detached')
func (s *shminfo) Shmat(id int, shmflg int) (unsafe.Pointer, error) {
	_addr, _, err := syscall.Syscall(syscall.SYS_SHMAT, uintptr(id), 0, uintptr(shmflg))
	if err != 0 {
		return nil, err
	}
	addr := unsafe.Pointer(_addr)

	s.Lock()
	if size, ok := s.id2Size[id]; ok {
		s.addr2Size[addr] = size
		s.addr2Id[addr] = id
	}
	s.Unlock()

	return addr, nil
}

// Shmdt ... Detach the address of the attachment point from the shared memory,
// effectively preventing the process from accessing this shared memory thereafter
func (s *shminfo) Shmdt(addr unsafe.Pointer) error {
	_, _, err := syscall.Syscall(syscall.SYS_SHMDT, uintptr(addr), 0, 0)
	if err != 0 {
		return err
	}

	s.Lock()
	delete(s.addr2Size, addr)
	delete(s.addr2Id, addr)
	s.Unlock()
	return nil
}

// Shmctl ...Full control over shared memory resources
// cmd:
// IPC_STAT: Retrieve the status of shared memory and copy the shmid_ds structure of the shared memory into the buffer, buf.
// IPC_SET: Change the status of the shared memory and copy the uid, gid, and mode in the shmid_ds structure pointed to by buf to the shmid_ds structure of the shared memory.
// IPC_RMID: Delete this shared memory
// buf: Shared memory management structure. For specific instructions, please refer to the shared memory kernel structure definition section.
func (s *shminfo) Shmctl(smid, cmd int) error {
	var buf uintptr = 0
	_, _, err := syscall.Syscall(syscall.SYS_SHMCTL, uintptr(smid), uintptr(cmd), buf)
	if err != 0 || cmd != IPC_RMID {
		return err
	}

	s.Lock()
	delete(s.id2Size, smid)
	for addr, id := range s.addr2Id {
		if id == smid {
			delete(s.addr2Id, addr)
			delete(s.addr2Size, addr)
		}
	}
	s.Unlock()
	return nil
}

// ShRead ... Read data from the shared memory
func (s *shminfo) ShRead(addr unsafe.Pointer) []byte {
	ptr := (*[4]byte)(addr)
	if ptr == nil {
		return nil
	}
	sizeBytes := *ptr
	size := int(binary.BigEndian.Uint32(sizeBytes[:])) - 4
	if size <= 0 {
		return []byte{}
	}
	buf := make([]byte, size)
	copy(buf, *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(addr) + 4,
		Len:  size,
		Cap:  size,
	})))
	runtime.KeepAlive(addr)
	return buf
}

// ShWrite ... Write data to the shared memory
func (s *shminfo) ShWrite(addr unsafe.Pointer, data []byte) error {
	size := 4 + len(data)
	s.RLock()
	maxSize := s.addr2Size[addr]
	s.RUnlock()
	if uint64(size) > maxSize {
		return fmt.Errorf("not enough space, (4 + %d) > %d", len(data), maxSize)
	}
	buf := make([]byte, size)
	// write the size into the first 4 bytes of buf in BigEndian format
	binary.BigEndian.PutUint32(buf, uint32(size))
	copy(buf[4:], data)
	// convert the address of the `buf` slice into a pointer.
	ptr := unsafe.Pointer(*(*uintptr)(unsafe.Pointer(&buf)))
	tmp := reflect.ArrayOf(size, reflect.TypeOf(byte(0)))
	// create a type `tmp` at memory address `addr`
	// and sets the value at that address to the value pointed to by `ptr`
	reflect.NewAt(tmp, addr).Elem().Set(reflect.NewAt(tmp, ptr).Elem())
	return nil
}
