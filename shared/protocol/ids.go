package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"sync/atomic"
)

var seq int64

func NewID() int64 {
	base := atomic.AddInt64(&seq, 1)
	var b [2]byte
	rand.Read(b[:])
	return (base << 16) | int64(binary.BigEndian.Uint16(b[:]))
}
