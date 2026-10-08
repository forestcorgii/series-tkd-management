package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorage_UploadAndDelete(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "stms_storage_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalStorage(tempDir, "/static/uploads/avatars")
	if err != nil {
		t.Fatalf("failed to initialize LocalStorage: %v", err)
	}

	content := "test image binary content"
	key := "test-avatar.png"

	url, err := storage.Upload(context.Background(), key, strings.NewReader(content), "image/png")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	expectedURL := "/static/uploads/avatars/test-avatar.png"
	if url != expectedURL {
		t.Errorf("expected url %q, got %q", expectedURL, url)
	}

	savedFile := filepath.Join(tempDir, key)
	data, err := os.ReadFile(savedFile)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected file content %q, got %q", content, string(data))
	}

	if err := storage.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := os.Stat(savedFile); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, but it still exists")
	}
}

func TestS3Storage_Signing(t *testing.T) {
	cfg := S3Config{
		Bucket:    "series-bucket",
		Endpoint:  "https://storage.railway.app",
		Region:    "auto",
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
		PublicURL: "https://cdn.example.com",
	}

	s3 := NewS3Storage(cfg)
	target, host, canonicalURI, err := s3.buildTarget("avatars/user-1.jpg")
	if err != nil {
		t.Fatalf("buildTarget failed: %v", err)
	}

	if host != "storage.railway.app" {
		t.Errorf("unexpected host: %s", host)
	}
	if canonicalURI != "/series-bucket/avatars/user-1.jpg" {
		t.Errorf("unexpected canonical URI: %s", canonicalURI)
	}
	if target != "https://storage.railway.app/series-bucket/avatars/user-1.jpg" {
		t.Errorf("unexpected target: %s", target)
	}
}

func TestLocalStorage_Get(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "stms_storage_get_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLocalStorage(tempDir, "/static/uploads/avatars")
	if err != nil {
		t.Fatalf("failed to initialize LocalStorage: %v", err)
	}

	content := "png-dummy-data"
	key := "avatar.png"
	_, err = storage.Upload(context.Background(), key, strings.NewReader(content), "image/png")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	rc, contentType, err := storage.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer rc.Close()

	if contentType != "image/png" {
		t.Errorf("expected contentType image/png, got %s", contentType)
	}

	readBytes, err := os.ReadFile(filepath.Join(tempDir, key))
	if err != nil {
		t.Fatalf("failed reading file: %v", err)
	}
	if string(readBytes) != content {
		t.Errorf("expected %q, got %q", content, string(readBytes))
	}
}

func TestExtractS3Key(t *testing.T) {
	bucket := "my-railway-bucket"

	tests := []struct {
		input    string
		expected string
	}{
		{"avatars/user-1.jpg", "avatars/user-1.jpg"},
		{"/avatars/user-1.jpg", "avatars/user-1.jpg"},
		{"/storage/avatars/user-1.jpg", "avatars/user-1.jpg"},
		{"storage/avatars/user-1.jpg", "avatars/user-1.jpg"},
		{"https://storage.railway.app/my-railway-bucket/avatars/user-1.jpg", "avatars/user-1.jpg"},
		{"http://storage.railway.app/my-railway-bucket/avatars/user-1.jpg", "avatars/user-1.jpg"},
	}

	for _, tc := range tests {
		actual := extractS3Key(tc.input, bucket)
		if actual != tc.expected {
			t.Errorf("extractS3Key(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestS3Storage_UploadReturnsStorageProxyWhenPublicURLOmitted(t *testing.T) {
	// Railway provides only bucket, access key, and secret key.
	// When PublicURL is empty, Upload must return /storage/{key}
	cfg := S3Config{
		Bucket:    "my-railway-bucket",
		Endpoint:  "https://storage.railway.app",
		Region:    "auto",
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
		PublicURL: "", // omitted in Railway
	}

	s3 := NewS3Storage(cfg)
	cleanKey := extractS3Key("avatars/user-999.png", s3.cfg.Bucket)
	expectedURL := "/storage/avatars/user-999.png"
	actualURL := "/storage/" + cleanKey

	if actualURL != expectedURL {
		t.Errorf("expected proxy URL %q, got %q", expectedURL, actualURL)
	}
}

func TestNewStorageFromEnv_S3Configuration(t *testing.T) {
	// Clean up environment after test
	origBucket := os.Getenv("BUCKET_NAME")
	origAccess := os.Getenv("AWS_ACCESS_KEY_ID")
	origSecret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	origEndpoint := os.Getenv("AWS_ENDPOINT_URL_S3")
	origRegion := os.Getenv("AWS_REGION")
	origPublicURL := os.Getenv("BUCKET_PUBLIC_URL")
	origPublicUR := os.Getenv("BUCKET_PUBLIC_UR")

	defer func() {
		os.Setenv("BUCKET_NAME", origBucket)
		os.Setenv("AWS_ACCESS_KEY_ID", origAccess)
		os.Setenv("AWS_SECRET_ACCESS_KEY", origSecret)
		os.Setenv("AWS_ENDPOINT_URL_S3", origEndpoint)
		os.Setenv("AWS_REGION", origRegion)
		os.Setenv("BUCKET_PUBLIC_URL", origPublicURL)
		os.Setenv("BUCKET_PUBLIC_UR", origPublicUR)
	}()

	os.Setenv("BUCKET_NAME", "custom-bucket")
	os.Setenv("AWS_ACCESS_KEY_ID", "custom-access-key")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "custom-secret-key")
	os.Setenv("AWS_ENDPOINT_URL_S3", "https://s3.custom-domain.com")
	os.Setenv("AWS_REGION", "ap-southeast-1")
	os.Setenv("BUCKET_PUBLIC_URL", "https://pub.example.com")
	os.Unsetenv("BUCKET_PUBLIC_UR")

	store := NewStorageFromEnv()
	s3Store, ok := store.(*S3Storage)
	if !ok {
		t.Fatalf("expected *S3Storage instance, got %T", store)
	}

	if s3Store.cfg.Bucket != "custom-bucket" {
		t.Errorf("expected Bucket custom-bucket, got %s", s3Store.cfg.Bucket)
	}
	if s3Store.cfg.AccessKey != "custom-access-key" {
		t.Errorf("expected AccessKey custom-access-key, got %s", s3Store.cfg.AccessKey)
	}
	if s3Store.cfg.SecretKey != "custom-secret-key" {
		t.Errorf("expected SecretKey custom-secret-key, got %s", s3Store.cfg.SecretKey)
	}
	if s3Store.cfg.Endpoint != "https://s3.custom-domain.com" {
		t.Errorf("expected Endpoint https://s3.custom-domain.com, got %s", s3Store.cfg.Endpoint)
	}
	if s3Store.cfg.Region != "ap-southeast-1" {
		t.Errorf("expected Region ap-southeast-1, got %s", s3Store.cfg.Region)
	}
	if s3Store.cfg.PublicURL != "https://pub.example.com" {
		t.Errorf("expected PublicURL https://pub.example.com, got %s", s3Store.cfg.PublicURL)
	}

	// Test BUCKET_PUBLIC_UR fallback
	os.Unsetenv("BUCKET_PUBLIC_URL")
	os.Setenv("BUCKET_PUBLIC_UR", "https://typo.example.com")
	store2 := NewStorageFromEnv()
	s3Store2, ok := store2.(*S3Storage)
	if !ok {
		t.Fatalf("expected *S3Storage, got %T", store2)
	}
	if s3Store2.cfg.PublicURL != "https://typo.example.com" {
		t.Errorf("expected PublicURL to read BUCKET_PUBLIC_UR, got %s", s3Store2.cfg.PublicURL)
	}
}


