package sqlite3vfsstorage

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/psanford/sqlite3vfs"
)

type StorageVFS struct {
	CacheHandler CacheHandler
	RoundTripper http.RoundTripper
}

type CacheHandler interface {
	Get(key interface{}) (value interface{}, ok bool)
	Add(key interface{}, value interface{})
}

func (vfs *StorageVFS) Open(name string, flags sqlite3vfs.OpenFlag) (sqlite3vfs.File, sqlite3vfs.OpenFlag, error) {
	bucket, key, err := parseURI(name)
	if err != nil {
		return nil, 0, err
	}
	storageBackend, err := GetBackend(name)
	if err != nil {
		return nil, 0, err
	}
	tf := &gcsFile{
		bucket:       bucket,
		key:          key,
		name:         name,
		cacheHandler: vfs.CacheHandler,
		roundTripper: vfs.RoundTripper,
		chunkSize:    4096 * 24, //this need to fit the page boundaries, default 4096
		Backend:      storageBackend,
	}

	return tf, flags, nil
}

func (vfs *StorageVFS) Delete(name string, dirSync bool) error {
	return sqlite3vfs.ReadOnlyError
}

func (vfs *StorageVFS) Access(name string, flag sqlite3vfs.AccessFlag) (bool, error) {
	if strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-journal") {
		return false, nil
	}
	return true, nil
}

func (vfs *StorageVFS) FullPathname(name string) string {
	return name
}

type gcsFile struct {
	bucket       string
	key          string
	name         string
	cacheHandler CacheHandler
	roundTripper http.RoundTripper
	chunkSize    int64
	Backend      StorageBackend
	fileSize     int64
}

func (tf *gcsFile) Close() error {
	return nil
}

func (tf *gcsFile) ReadAt(p []byte, off int64) (int, error) {
	offStart := off % tf.chunkSize
	chunkStart := tf.chunkSize * int64(math.Floor(float64(off)/float64(tf.chunkSize)))

	if tf.cacheHandler != nil {
		buf, ok := tf.cacheHandler.Get(fmt.Sprintf("%s-%d", tf.name, chunkStart))
		if ok {
			copy(p, buf.([]byte)[offStart:])
			return len(p), nil
		}
	}
	rc, err := tf.Backend.RangeReader(tf.name, chunkStart, chunkStart+tf.chunkSize)
	if err != nil {
		return 0, fmt.Errorf("Object(%q).NewReader: %v", tf.key, err)
	}
	defer rc.Close()
	bytesToRead := tf.chunkSize
	if chunkStart+tf.chunkSize > tf.fileSize {
		bytesToRead = tf.fileSize - chunkStart
	}

	fullbuf := make([]byte, bytesToRead)
	n, err := io.ReadFull(rc, fullbuf)
	if err != nil {
		fmt.Printf("io.ReadFull: %v", err)
		return 0, err
	}

	copy(p, fullbuf[offStart:])
	if tf.cacheHandler != nil {
		tf.cacheHandler.Add(fmt.Sprintf("%s-%d", tf.name, chunkStart), fullbuf)
	}

	return n, nil
}

func (tf *gcsFile) WriteAt(b []byte, off int64) (n int, err error) {
	return 0, sqlite3vfs.ReadOnlyError
}

func (tf *gcsFile) Truncate(size int64) error {
	return sqlite3vfs.ReadOnlyError
}

func (tf *gcsFile) Sync(flag sqlite3vfs.SyncType) error {
	return nil
}

func (tf *gcsFile) FileSize() (int64, error) {
	if tf.fileSize != 0 {
		return tf.fileSize, nil
	}
	fsize, err := tf.Backend.FileSize(tf.name)
	if err != nil {
		return 0, err
	}
	tf.fileSize = fsize
	return fsize, nil
}

func (tf *gcsFile) Lock(elock sqlite3vfs.LockType) error {
	return nil
}

func (tf *gcsFile) Unlock(elock sqlite3vfs.LockType) error {
	return nil
}

func (tf *gcsFile) CheckReservedLock() (bool, error) {
	return false, nil
}

func (tf *gcsFile) SectorSize() int64 {
	return 0
}

func (tf *gcsFile) DeviceCharacteristics() sqlite3vfs.DeviceCharacteristic {
	return sqlite3vfs.IocapImmutable
}
