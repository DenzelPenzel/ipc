package ipc

import (
	"errors"
	"syscall"
	"unsafe"
)

const (
	msgSize = 1 << 13
	/* Define options for message queue functions.  */
	MSG_BLOCK = 0
)

// GetMsg ... Retrieves the message queue identifier if it exists,
// or creates a new message queue object if not found, and returns the corresponding identifier
func GetMsg(key uint64, msgflag int) (int, error) {
	id, _, err := syscall.Syscall(syscall.SYS_MSGGET, uintptr(key), uintptr(msgflag), 0)
	if err != 0 {
		return 0, err
	}
	return int(id), nil
}

func receiveMsg(msgid int, msgp uintptr, msgtyp uint, msgflg int) (uintptr, error) {
	r, _, err := syscall.Syscall6(
		syscall.SYS_MSGRCV,
		uintptr(msgid),
		msgp,
		uintptr(msgSize),
		uintptr(msgtyp),
		uintptr(msgflg),
		0,
	)
	if err != 0 {
		return r, err
	}
	return r, nil
}

func ReceiveMsg(msgid int, msgType uint, flag int) ([]byte, error) {
	m := Message{Mtype: msgType}
	readLen, err := receiveMsg(msgid, uintptr(unsafe.Pointer(&m)), msgType, flag)
	if err != nil {
		return nil, err
	}
	return m.Mtext[:readLen], nil
}

func sendMsg(msgid int, msgp uintptr, msgsz int, msgflg int) error {
	_, _, err := syscall.Syscall6(
		syscall.SYS_MSGSND,
		uintptr(msgid),
		msgp,
		uintptr(msgsz),
		uintptr(msgflg),
		0,
		0,
	)
	if err != 0 {
		return err
	}
	return nil
}

func SendMsg(msgid int, msgType uint, msgText []byte, flags int) error {
	if len(msgText) > msgSize {
		return errors.New("[error] message length too long")
	}
	m := Message{Mtype: msgType}
	copy(m.Mtext[:], msgText)
	return sendMsg(msgid, uintptr(unsafe.Pointer(&m)), len(msgText), flags)
}

// RemoveMsg ... Removes the message queue associated with the given identifier
func RemoveMsg(msgid int) error {
	_, _, err := syscall.Syscall(syscall.SYS_MSGCTL, uintptr(msgid), IPC_RMID, 0)
	if err != 0 {
		return err
	}
	return nil
}

type Message struct {
	Mtype uint
	Mtext [msgSize]byte
}

// The msgp argument is a pointer to a caller-defined structure of the
// following general form
type msgbuf struct {
	Mtype uint
	Mtext [0]byte
}
