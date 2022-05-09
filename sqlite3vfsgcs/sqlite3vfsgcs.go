package sqlite3vfsgcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/psanford/sqlite3vfs"
)

func getObjectSize(bucket string, key string) (int64, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return 0, fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	o := client.Bucket(bucket).Object(key)
	attrs, err := o.Attrs(ctx)
	if err != nil {
		return 0, fmt.Errorf("Object(%q).Attrs: %v", key, err)
	}
	return attrs.Size, nil
}

type GcsVFS struct {
	CacheHandler CacheHandler
	RoundTripper http.RoundTripper
}

type CacheHandler interface {
	Get(key interface{}) (value interface{}, ok bool)
	Add(key interface{}, value interface{})
}

func (vfs *GcsVFS) Open(name string, flags sqlite3vfs.OpenFlag) (sqlite3vfs.File, sqlite3vfs.OpenFlag, error) {
	u, err := url.Parse(name)
	if err != nil {
		panic(err)
	}
	tf := &gcsFile{
		bucket:       u.Host,
		key:          u.Path[1:],
		name:         name,
		cacheHandler: vfs.CacheHandler,
		roundTripper: vfs.RoundTripper,
		chunkSize:    4096 * 24, //this need to fit the page boundaries, default 4096
	}

	return tf, flags, nil
}

func (vfs *GcsVFS) Delete(name string, dirSync bool) error {
	return sqlite3vfs.ReadOnlyError
}

func (vfs *GcsVFS) Access(name string, flag sqlite3vfs.AccessFlag) (bool, error) {
	if strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-journal") {
		return false, nil
	}

	return true, nil
}

func (vfs *GcsVFS) FullPathname(name string) string {
	return name
}

type gcsFile struct {
	bucket       string
	key          string
	name         string
	cacheHandler CacheHandler
	roundTripper http.RoundTripper
	chunkSize    int64
}

func (tf *gcsFile) Close() error {
	return nil
}

func (tf *gcsFile) client() *http.Client {
	if tf.roundTripper == nil {
		return http.DefaultClient
	}
	return &http.Client{
		Transport: tf.roundTripper,
	}
}

var hits = 0
var misses = 0

func (tf *gcsFile) ReadAt(p []byte, off int64) (int, error) {

	offStart := off % tf.chunkSize
	chunkStart := tf.chunkSize * int64(math.Floor(float64(off)/float64(tf.chunkSize)))

	if tf.cacheHandler != nil {
		buf, ok := tf.cacheHandler.Get(fmt.Sprintf("%s-%d", tf.name, chunkStart))
		if ok {
			hits += 1
			// fmt.Printf("Cache hit: %v\n", fmt.Sprintf("%s-%d", tf.name, chunkStart))

			copy(p, buf.([]byte)[offStart:])
			// fmt.Printf("P cache Bytes %v to %v of blob \n", chunkStart, chunkStart+tf.chunkSize)
			return len(p), nil
		} else {
			misses += 1
		}
		// fmt.Printf("Cache miss: %v hits: %v\n", misses, hits)
	}
	// fmt.Printf("ReadAt: %v - %v\n", off, len(p))

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return 0, fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	//todo what about end of the file? will that just work out? probably...
	// fmt.Printf("ReadAt: %v - %v\n", chunkStart, tf.chunkSize)
	rc, err := client.Bucket(tf.bucket).Object(tf.key).NewRangeReader(ctx, chunkStart, tf.chunkSize)
	if err != nil {
		return 0, fmt.Errorf("Object(%q).NewReader: %v", tf.key, err)
	}
	defer rc.Close()
	fullbuf := make([]byte, tf.chunkSize)
	n, err := io.ReadFull(rc, fullbuf)
	if err != nil {
		fmt.Printf("io.ReadFull: %v", err)
		return 0, err
	}

	copy(p, fullbuf[offStart:])
	if tf.cacheHandler != nil {
		tf.cacheHandler.Add(fmt.Sprintf("%s-%d", tf.name, chunkStart), fullbuf)
	}
	// fmt.Printf("P Bytes %v to %v of blob \n", off, off+int64(n))
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

var invalidContentRangeErr = errors.New("invalid Content-Range response")

func (tf *gcsFile) FileSize() (int64, error) {

	size, err := getObjectSize(tf.bucket, tf.key)
	if err != nil {
		fmt.Printf("getObjectSize: %v", err)
		return 0, err
	}

	return size, nil
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
