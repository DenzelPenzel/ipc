package ipc

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSharedMem_Write(t *testing.T) {
	key, err := Ftok("test/a", 5)
	require.NoError(t, err)
	s := NewShm()
	shmid, err := s.Shmget(key, 32, IPC_CREAT|IPC_RW)
	require.NoError(t, err)
	// attach action
	shmaddr, err := s.Shmat(shmid, SHM_REMAP)
	require.NoError(t, err)
	// write action
	s.Shmwrite(shmaddr, []byte("test"))
	require.NoError(t, err)
	// detach action
	s.Shmdt(shmaddr)
	require.NoError(t, err)
}

func TestSharedMem_Read(t *testing.T) {
	key, err := Ftok("test/a", 5)
	require.NoError(t, err)
	s := NewShm()

	shmid, err := s.Shmget(key, 0, IPC_R)
	require.NoError(t, err)

	defer func() {
		err := s.Shmctl(shmid, IPC_RMID)
		if err != nil {
			t.Fatal(err)
		}
	}()

	shmaddr, err := s.Shmat(shmid, SHM_REMAP)
	require.NoError(t, err)

	data := s.Shmread(shmaddr)
	require.Equal(t, []byte("test"), data)
}

func TestSharedMem_Shmat(t *testing.T) {
	done := make(chan interface{})
	want := "test data"
	s := NewShm()

	go func() {
		key, err := Ftok("test/b", 5)
		if err != nil {
			panic(err)
		}
		shmid, err := s.Shmget(key, 32, IPC_CREAT|IPC_RW)
		shmaddr, err := s.Shmat(shmid, SHM_REMAP)
		if err != nil {
			t.Error(err)
		}
		s.Shmwrite(shmaddr, []byte(want))
		time.Sleep(time.Millisecond * 100)
		// detach action
		s.Shmdt(shmaddr)
		close(done)
	}()

	time.Sleep(time.Millisecond * 100)
	key, err := Ftok("test/b", 5)
	require.NoError(t, err)

	shmid, err := s.Shmget(key, 0, IPC_R)
	require.NoError(t, err)
	shmaddr, err := s.Shmat(shmid, SHM_REMAP)
	require.NoError(t, err)

	got := s.Shmread(shmaddr)

	defer s.Shmctl(shmid, IPC_RMID)

	require.Equal(t, []byte(want), got)

	<-done
}
