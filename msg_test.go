package ipc

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

var want = strings.Repeat("1", 128)

func TestMsg_SendAndRec(t *testing.T) {
	done := make(chan struct{})

	go func() {
		key, err := Ftok("test", 5)
		require.NoError(t, err)

		msgid, err := GetMsg(key, IPC_CREAT|IPC_RW)
		require.NoError(t, err)

		defer RemoveMsg(msgid)

		err = SendMsg(msgid, 1, []byte(want), MSG_BLOCK)
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 200)
		close(done)
	}()

	time.Sleep(time.Millisecond * 100)

	key, err := Ftok("test", 5)
	require.NoError(t, err)
	msgid, err := GetMsg(key, IPC_R)
	require.NoError(t, err)

	got, err := ReceiveMsg(msgid, 0, IPC_NOWAIT)
	require.NoError(t, err)
	require.Equal(t, want, string(got))

	<-done
}
