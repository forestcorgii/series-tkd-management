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
// Prioritized Environment Variables & Aliases:
//   - BUCKET_NAME (aliases: BUCKET, AWS_S3_BUCKET_NAME, AWS_BUCKET_NAME, S3_BUCKET_NAME, S3_BUCKET, TIGRIS_BUCKET, RAILWAY_BUCKET_NAME, RAILWAY_BUCKET)
//   - AWS_ACCESS_KEY_ID (aliases: ACCESS_KEY_ID, BUCKET_ACCESS_KEY_ID, AWS_ACCESS_KEY, TIGRIS_ACCESS_KEY_ID)
//   - AWS_SECRET_ACCESS_KEY (aliases: SECRET_ACCESS_KEY, BUCKET_SECRET_ACCESS_KEY, AWS_SECRET_KEY, TIGRIS_SECRET_ACCESS_KEY)
//   - AWS_ENDPOINT_URL_S3 (aliases: ENDPOINT, BUCKET_ENDPOINT, AWS_ENDPOINT_URL, AWS_ENDPOINT, S3_ENDPOINT, TIGRIS_ENDPOINT; defaults to https://storage.railway.app)
//   - AWS_REGION (aliases: REGION, AWS_DEFAULT_REGION, BUCKET_REGION, TIGRIS_REGION; defaults to auto)
//   - BUCKET_PUBLIC_URL (optional; aliases: BUCKET_PUBLIC_UR, S3_PUBLIC_URL, PUBLIC_URL)
func NewStorageFromEnv() FileStorage {
	bucket := getFirstEnv("BUCKET", "BUCKET_NAME", "AWS_S3_BUCKET_NAME", "AWS_BUCKET_NAME", "S3_BUCKET_NAME", "S3_BUCKET", "TIGRIS_BUCKET", "RAILWAY_BUCKET_NAME", "RAILWAY_BUCKET")
	accessKey := getFirstEnv("ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID", "BUCKET_ACCESS_KEY_ID", "AWS_ACCESS_KEY", "TIGRIS_ACCESS_KEY_ID")
	secretKey := getFirstEnv("SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY", "BUCKET_SECRET_ACCESS_KEY", "AWS_SECRET_KEY", "TIGRIS_SECRET_ACCESS_KEY")
	endpoint := getFirstEnv("ENDPOINT", "BUCKET_ENDPOINT", "AWS_ENDPOINT_URL_S3", "AWS_ENDPOINT_URL", "AWS_ENDPOINT", "S3_ENDPOINT", "TIGRIS_ENDPOINT")
	region := getFirstEnv("REGION", "AWS_REGION", "AWS_DEFAULT_REGION", "BUCKET_REGION", "TIGRIS_REGION")
	publicURL := getFirstEnv("BUCKET_PUBLIC_URL", "BUCKET_PUBLIC_UR", "S3_PUBLIC_URL", "PUBLIC_URL")
	forcePathStyle := strings.ToLower(getFirstEnv("FORCE_PATH_STYLE", "S3_FORCE_PATH_STYLE", "AWS_S3_FORCE_PATH_STYLE")) == "true"

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
			Bucket:         bucket,
			Endpoint:       endpoint,
			Region:         region,
			AccessKey:      accessKey,
			SecretKey:      secretKey,
			PublicURL:      publicURL,
			ForcePathStyle: forcePathStyle,
		})
	}

	if bucket != "" || accessKey != "" || secretKey != "" {
		log.Printf("⚠️ WARNING: Incomplete S3/Bucket credentials! (bucket=%q, accessKeySet=%v, secretKeySet=%v). Falling back to ephemeral local storage.",
			bucket, accessKey != "", secretKey != "")
	} else if os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("RAILWAY_PROJECT_ID") != "" {
		log.Printf("⚠️ NOTICE [Railway]: No S3 bucket credentials found on this service. Falling back to local storage (ephemeral).")
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
		val := strings.TrimSpace(os.Getenv(k))
		if val == "" {
			continue
		}
		val = strings.Trim(val, "\"'")
		val = strings.TrimSpace(val)
		if strings.HasPrefix(val, "${{") && strings.HasSuffix(val, "}}") {
			log.Printf("⚠️ Warning: Environment variable %s contains unresolved Railway reference %q. Check Railway service name and variable key.", k, val)
			continue
		}
		if val != "" {
			return val
		}
	}
	return ""
}
