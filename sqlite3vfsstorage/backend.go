package sqlite3vfsstorage

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go/service/s3"
)

type StorageBackend interface {
	// Returns file size in bytes of object
	FileSize(name string) (int64, error)
	// Returns a reader for the range of the object
	RangeReader(name string, start, end int64) (io.ReadCloser, error)
}

func parseURI(uri string) (bucket, key string, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}
	bucket = u.Host
	key = u.Path[1:]
	return bucket, key, nil
}

func GetBackend(uri string) (StorageBackend, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "gs":
		return newGCSBackend()
	case "s3":
		return newS3Backend()
	default:
		return nil, error(fmt.Errorf("unknown scheme: %s", u.Scheme))
	}
}

// GCSBackend implements StorageBackend for Google Cloud Storage
type GCSBackend struct {
	// Client is a Google Cloud Storage client
	Client *storage.Client
}

func newGCSBackend() (*GCSBackend, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &GCSBackend{Client: client}, nil
}

func (b *GCSBackend) FileSize(name string) (int64, error) {
	bucket, key, err := parseURI(name)
	if err != nil {
		return 0, err
	}
	o := b.Client.Bucket(bucket).Object(key)
	attrs, err := o.Attrs(context.Background())
	if err != nil {
		return 0, err
	}
	return attrs.Size, nil
}

func (b *GCSBackend) RangeReader(name string, start, end int64) (io.ReadCloser, error) {
	bucket, key, err := parseURI(name)
	if err != nil {
		return nil, err
	}
	o := b.Client.Bucket(bucket).Object(key)
	return o.NewRangeReader(context.Background(), start, end-start+1)
}

// S3Backend implements StorageBackend for Amazon S3
type S3Backend struct {
	// Client is an Amazon S3 client
	Client *s3.S3
}

func newS3Backend() (*S3Backend, error) {
	return &S3Backend{
		Client: s3.New(nil),
	}, nil
}

func (b *S3Backend) FileSize(name string) (int64, error) {
	bucket, key, err := parseURI(name)
	if err != nil {
		return 0, err
	}
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	output, err := b.Client.HeadObject(input)
	if err != nil {
		return 0, err
	}
	return *output.ContentLength, nil
}

func (b *S3Backend) RangeReader(name string, start, end int64) (io.ReadCloser, error) {
	bucket, key, err := parseURI(name)
	if err != nil {
		return nil, err
	}
	input := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	input.SetRange(fmt.Sprintf("bytes=%d-%d", start, end))
	output, err := b.Client.GetObject(input)
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}
