package backend

import (
	"sync"
)

type MemoryBackend struct {
	memory []byte
	lock   sync.Mutex
}

func NewMemoryBackend(memory []byte) *MemoryBackend {
	return &MemoryBackend{memory, sync.Mutex{}}
}

func (b *MemoryBackend) ReadAt(p []byte, off int64) (n int, err error) {
	b.lock.Lock()

	n = copy(p, b.memory[off:off+int64(len(p))])

	b.lock.Unlock()

	return
}

func (b *MemoryBackend) WriteAt(p []byte, off int64) (n int, err error) {
	b.lock.Lock()

	n = copy(b.memory[off:off+int64(len(p))], p)

	b.lock.Unlock()

	return
}

func (b *MemoryBackend) Size() (int64, error) {
	return int64(len(b.memory)), nil
}

func (b *MemoryBackend) Sync() error {
	return nil
}
