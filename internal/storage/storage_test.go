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

