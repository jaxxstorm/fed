package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const MaxArtifactBytes = 16 << 20

var artifactKey = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Artifacts is deliberately unaware of HTTP. Domain services must authorize
// access to the evidence record before calling Get.
type Artifacts interface {
	Put(context.Context, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
}

type Files struct{ Directory string }

func (f Files) Put(ctx context.Context, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(data) > MaxArtifactBytes {
		return "", errors.New("artifact too large")
	}
	if err := os.MkdirAll(f.Directory, 0700); err != nil {
		return "", err
	}
	key := Hash(data)
	tmp, err := os.CreateTemp(f.Directory, ".upload-")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(tmp.Name(), filepath.Join(f.Directory, key)); err != nil {
		return "", err
	}
	return key, nil
}

func (f Files) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !artifactKey.MatchString(key) {
		return nil, errors.New("invalid artifact key")
	}
	root, err := os.OpenRoot(f.Directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(key)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readArtifact(file, key)
}

type S3 struct {
	Client *s3.Client
	Bucket string
}

func (s S3) Put(ctx context.Context, data []byte) (string, error) {
	if len(data) > MaxArtifactBytes {
		return "", errors.New("artifact too large")
	}
	key := Hash(data)
	_, err := s.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key), Body: bytes.NewReader(data), ContentType: aws.String("application/octet-stream")})
	return key, err
}

func (s S3) Get(ctx context.Context, key string) ([]byte, error) {
	if !artifactKey.MatchString(key) {
		return nil, errors.New("invalid artifact key")
	}
	result, err := s.Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.Bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	return readArtifact(result.Body, key)
}

func readArtifact(r io.Reader, key string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxArtifactBytes || Hash(data) != key {
		return nil, errors.New("artifact integrity check failed")
	}
	return data, nil
}
