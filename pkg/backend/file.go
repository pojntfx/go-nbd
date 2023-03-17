package backend

import (
	"os"
	"sync"
)

type FileBackend struct {
	file *os.File
	lock sync.RWMutex
}

func NewFileBackend(file *os.File) *FileBackend {
	return &FileBackend{file, sync.RWMutex{}}
}

func (b *FileBackend) ReadAt(p []byte, off int64) (n int, err error) {
	b.lock.RLock()

	n, err = b.file.ReadAt(p, off)

	b.lock.RUnlock()

	return
}

func (b *FileBackend) WriteAt(p []byte, off int64) (n int, err error) {
	b.lock.Lock()

	n, err = b.file.WriteAt(p, off)

	b.lock.Unlock()

	return
}

func (b *FileBackend) Size() (int64, error) {
	stat, err := b.file.Stat()
	if err != nil {
		return -1, err
	}

	return stat.Size(), nil
}

func (b *FileBackend) Sync() error {
	return b.file.Sync()
}
