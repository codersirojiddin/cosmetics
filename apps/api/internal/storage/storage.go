// Package storage abstracts file storage: local disk for development, S3-compatible
// (Cloudflare R2) for production. Product images are public, receipts are private.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
)

type Visibility int

const (
	Public Visibility = iota
	Private
)

var ErrNotFound = errors.New("storage: object not found")

type Storage interface {
	Put(ctx context.Context, vis Visibility, key string, data []byte, contentType string) error
	Get(ctx context.Context, vis Visibility, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, vis Visibility, key string) error
	// PublicURL returns the browser-facing URL of a public object.
	PublicURL(key string) string
}

func validKey(key string) error {
	if key == "" || strings.Contains(key, "..") || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return fmt.Errorf("storage: invalid key %q", key)
	}
	return nil
}

// ---------- local disk ----------

type Local struct {
	root       string
	publicBase string
}

// NewLocal stores files under root/public and root/private. publicBase is the URL prefix
// (without trailing slash) under which root/public is served, e.g. http://localhost:8080/files.
func NewLocal(root, publicBase string) (*Local, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, d := range []string{"public", "private"} {
		if err := os.MkdirAll(filepath.Join(abs, d), 0o755); err != nil {
			return nil, err
		}
	}
	return &Local{root: abs, publicBase: strings.TrimRight(publicBase, "/")}, nil
}

func (l *Local) PublicDir() string { return filepath.Join(l.root, "public") }

func (l *Local) path(vis Visibility, key string) (string, error) {
	if err := validKey(key); err != nil {
		return "", err
	}
	dir := "public"
	if vis == Private {
		dir = "private"
	}
	return filepath.Join(l.root, dir, filepath.FromSlash(key)), nil
}

func (l *Local) Put(_ context.Context, vis Visibility, key string, data []byte, _ string) error {
	p, err := l.path(vis, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (l *Local) Get(_ context.Context, vis Visibility, key string) (io.ReadCloser, error) {
	p, err := l.path(vis, key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (l *Local) Delete(_ context.Context, vis Visibility, key string) error {
	p, err := l.path(vis, key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (l *Local) PublicURL(key string) string { return l.publicBase + "/" + key }

// ---------- S3 / R2 ----------

type S3 struct {
	c          *minio.Client
	pub, priv  string
	publicBase string
}

func NewS3(cfg config.S3Config) (*S3, error) {
	endpoint := cfg.Endpoint
	secure := true
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		endpoint, secure = u.Host, u.Scheme != "http"
	}
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}
	return &S3{c: c, pub: cfg.PublicBucket, priv: cfg.PrivateBucket, publicBase: cfg.PublicBaseURL}, nil
}

func (s *S3) bucket(vis Visibility) string {
	if vis == Private {
		return s.priv
	}
	return s.pub
}

func (s *S3) Put(ctx context.Context, vis Visibility, key string, data []byte, contentType string) error {
	if err := validKey(key); err != nil {
		return err
	}
	opts := minio.PutObjectOptions{ContentType: contentType}
	if vis == Public {
		opts.CacheControl = "public, max-age=31536000, immutable"
	}
	_, err := s.c.PutObject(ctx, s.bucket(vis), key, bytes.NewReader(data), int64(len(data)), opts)
	return err
}

func (s *S3) Get(ctx context.Context, vis Visibility, key string) (io.ReadCloser, error) {
	if err := validKey(key); err != nil {
		return nil, err
	}
	obj, err := s.c.GetObject(ctx, s.bucket(vis), key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return obj, nil
}

func (s *S3) Delete(ctx context.Context, vis Visibility, key string) error {
	if err := validKey(key); err != nil {
		return err
	}
	return s.c.RemoveObject(ctx, s.bucket(vis), key, minio.RemoveObjectOptions{})
}

func (s *S3) PublicURL(key string) string { return s.publicBase + "/" + key }
