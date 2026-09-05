package storage

import (
	"context"
	"errors"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Region    string
	UseSSL    bool
}

type S3Store struct {
	client *minio.Client
}

func NewS3Store(config S3Config) (*S3Store, error) {
	if config.Endpoint == "" || config.AccessKey == "" || config.SecretKey == "" {
		return nil, errors.New("S3 endpoint and credentials are required")
	}
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, err
	}
	return &S3Store{client: client}, nil
}

func (s *S3Store) Put(ctx context.Context, bucket, key, contentType string, body io.Reader, size int64) (ObjectInfo, error) {
	result, err := s.client.PutObject(ctx, bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Bucket: bucket, Key: key, ETag: result.ETag, ContentType: contentType, Size: result.Size}, nil
}

func (s *S3Store) Get(ctx context.Context, bucket, key string) (ObjectReader, error) {
	object, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return ObjectReader{}, err
	}
	info, err := object.Stat()
	if err != nil {
		_ = object.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" || minio.ToErrorResponse(err).Code == "NoSuchBucket" {
			return ObjectReader{}, ErrObjectNotFound
		}
		return ObjectReader{}, err
	}
	return ObjectReader{ObjectInfo: ObjectInfo{Bucket: bucket, Key: key, ETag: info.ETag, ContentType: info.ContentType, Size: info.Size, LastModified: info.LastModified}, Body: object}, nil
}

func (s *S3Store) Delete(ctx context.Context, bucket, key string) error {
	return s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3Store) List(ctx context.Context, bucket, prefix string, limit, offset int) ([]ObjectInfo, error) {
	items := make([]ObjectInfo, 0)
	skipped := 0
	for item := range s.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if item.Err != nil {
			return nil, item.Err
		}
		if skipped < offset {
			skipped++
			continue
		}
		items = append(items, ObjectInfo{Bucket: bucket, Key: item.Key, ETag: item.ETag, ContentType: item.ContentType, Size: item.Size, LastModified: item.LastModified})
		if limit >= 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}
