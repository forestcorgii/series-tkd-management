package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// FileStorage defines the interface for storing, retrieving, and deleting uploaded files.
type FileStorage interface {
	Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, key string) error
}

// LocalStorage stores uploaded files on the local filesystem.
type LocalStorage struct {
	BaseDir string
	URLPath string
}

func NewLocalStorage(baseDir, urlPath string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory %s: %w", baseDir, err)
	}
	return &LocalStorage{
		BaseDir: baseDir,
		URLPath: strings.TrimRight(urlPath, "/"),
	}, nil
}

func (l *LocalStorage) Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	cleanKey := filepath.Base(key)
	filePath := filepath.Join(l.BaseDir, cleanKey)

	out, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, data); err != nil {
		return "", fmt.Errorf("failed to write local file: %w", err)
	}

	return fmt.Sprintf("%s/%s", l.URLPath, cleanKey), nil
}

func (l *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	cleanKey := filepath.Base(key)
	filePath := filepath.Join(l.BaseDir, cleanKey)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}

	ext := strings.ToLower(filepath.Ext(cleanKey))
	contentType := "application/octet-stream"
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".webp":
		contentType = "image/webp"
	case ".gif":
		contentType = "image/gif"
	}
	return f, contentType, nil
}

func (l *LocalStorage) Delete(ctx context.Context, key string) error {
	cleanKey := filepath.Base(key)
	filePath := filepath.Join(l.BaseDir, cleanKey)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// NewStorageFromEnv initializes S3 storage if Railway Bucket or AWS S3 credentials are configured;
// otherwise it falls back to LocalStorage under web/static/uploads/avatars.
//
// Prioritized Environment Variables:
//   - BUCKET_NAME (alias: BUCKET, AWS_S3_BUCKET_NAME)
//   - AWS_ACCESS_KEY_ID (alias: ACCESS_KEY_ID)
//   - AWS_SECRET_ACCESS_KEY (alias: SECRET_ACCESS_KEY)
//   - AWS_ENDPOINT_URL_S3 (alias: ENDPOINT, AWS_ENDPOINT_URL; defaults to https://storage.railway.app)
//   - AWS_REGION (alias: REGION, AWS_DEFAULT_REGION; defaults to auto)
//   - BUCKET_PUBLIC_URL (optional; alias: BUCKET_PUBLIC_UR, S3_PUBLIC_URL, PUBLIC_URL)
func NewStorageFromEnv() FileStorage {
	bucket := getFirstEnv("BUCKET_NAME", "BUCKET", "AWS_S3_BUCKET_NAME", "AWS_BUCKET_NAME", "S3_BUCKET_NAME", "S3_BUCKET")
	accessKey := getFirstEnv("AWS_ACCESS_KEY_ID", "ACCESS_KEY_ID", "BUCKET_ACCESS_KEY_ID", "AWS_ACCESS_KEY")
	secretKey := getFirstEnv("AWS_SECRET_ACCESS_KEY", "SECRET_ACCESS_KEY", "BUCKET_SECRET_ACCESS_KEY", "AWS_SECRET_KEY")
	endpoint := getFirstEnv("AWS_ENDPOINT_URL_S3", "ENDPOINT", "AWS_ENDPOINT_URL", "BUCKET_ENDPOINT", "AWS_ENDPOINT", "S3_ENDPOINT")
	region := getFirstEnv("AWS_REGION", "REGION", "AWS_DEFAULT_REGION", "BUCKET_REGION")
	publicURL := getFirstEnv("BUCKET_PUBLIC_URL", "BUCKET_PUBLIC_UR", "S3_PUBLIC_URL", "PUBLIC_URL")

	// Railway Buckets default to storage.railway.app and region "auto" if not explicitly specified
	if endpoint == "" {
		endpoint = "https://storage.railway.app"
	}
	if region == "" {
		region = "auto"
	}

	if bucket != "" && accessKey != "" && secretKey != "" {
		log.Printf("☁️ Railway Object Storage / S3 enabled for bucket: %s (endpoint: %s, region: %s)", bucket, endpoint, region)
		if publicURL != "" {
			log.Printf("🌐 Bucket public URL configured: %s", publicURL)
		} else {
			log.Printf("🔒 Private bucket mode: serving assets via internal signed proxy (/storage/{key})")
		}
		return NewS3Storage(S3Config{
			Bucket:    bucket,
			Endpoint:  endpoint,
			Region:    region,
			AccessKey: accessKey,
			SecretKey: secretKey,
			PublicURL: publicURL,
		})
	}

	// Local disk fallback
	uploadDir := "web/static/uploads/avatars"
	for _, c := range []string{"web/static/uploads/avatars", "../web/static/uploads/avatars", "../../web/static/uploads/avatars"} {
		parent := filepath.Dir(c)
		if fi, err := os.Stat(parent); err == nil && fi.IsDir() {
			uploadDir = c
			break
		}
	}

	localStorage, err := NewLocalStorage(uploadDir, "/static/uploads/avatars")
	if err != nil {
		log.Printf("⚠️ Warning: could not initialize local uploads directory %s: %v", uploadDir, err)
	} else {
		log.Printf("📁 Local storage fallback initialized at: %s", uploadDir)
	}
	return localStorage
}

func getFirstEnv(keys ...string) string {
	for _, k := range keys {
		if val := strings.TrimSpace(os.Getenv(k)); val != "" {
			return val
		}
	}
	return ""
}
