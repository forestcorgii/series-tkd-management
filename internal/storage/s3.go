package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type S3Config struct {
	Bucket         string
	Endpoint       string
	Region         string
	AccessKey      string
	SecretKey      string
	PublicURL      string
	ForcePathStyle bool
}

type S3Storage struct {
	cfg        S3Config
	httpClient *http.Client
}

func NewS3Storage(cfg S3Config) *S3Storage {
	if cfg.Region == "" {
		cfg.Region = "auto"
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", cfg.Region)
	}
	return &S3Storage{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func extractS3Key(key, bucket string) string {
	clean := strings.TrimLeft(key, "/")
	clean = strings.TrimPrefix(clean, "storage/")
	if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") {
		if u, err := url.Parse(clean); err == nil {
			clean = strings.TrimLeft(u.Path, "/")
			if bucket != "" {
				clean = strings.TrimPrefix(clean, bucket+"/")
			}
		}
	}
	return clean
}

func (s *S3Storage) Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	cleanKey := extractS3Key(key, s.cfg.Bucket)

	payload, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("failed to read upload payload: %w", err)
	}

	targetURL, host, canonicalURI, err := s.buildTarget(cleanKey)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, targetURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)

	// Sign request with AWS SigV4
	now := time.Now().UTC()
	s.signRequest(req, http.MethodPut, host, canonicalURI, payload, contentType, now)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute s3 put request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("s3 upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Determine public URL
	if s.cfg.PublicURL != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(s.cfg.PublicURL, "/"), cleanKey), nil
	}

	// Railway Buckets are private S3-compatible buckets with no public URL by default.
	// Return the proxy URL /storage/{key} which serves content via signed SigV4 GET.
	return fmt.Sprintf("/storage/%s", cleanKey), nil
}

func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	cleanKey := extractS3Key(key, s.cfg.Bucket)
	targetURL, host, canonicalURI, err := s.buildTarget(cleanKey)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create s3 get request: %w", err)
	}

	now := time.Now().UTC()
	s.signRequest(req, http.MethodGet, host, canonicalURI, nil, "", now)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute s3 get request: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, "", os.ErrNotExist
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, "", fmt.Errorf("s3 get failed with status %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return resp.Body, contentType, nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	cleanKey := extractS3Key(key, s.cfg.Bucket)
	targetURL, host, canonicalURI, err := s.buildTarget(cleanKey)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, targetURL, nil)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	s.signRequest(req, http.MethodDelete, host, canonicalURI, nil, "", now)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func isIPOrLocalhost(host string) bool {
	h := host
	if colon := strings.Index(h, ":"); colon != -1 {
		h = h[:colon]
	}
	if h == "localhost" {
		return true
	}
	return net.ParseIP(h) != nil
}

func (s *S3Storage) buildTarget(key string) (targetURL, host, canonicalURI string, err error) {
	ep := s.cfg.Endpoint
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "https://" + ep
	}

	parsed, err := url.Parse(ep)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid s3 endpoint %s: %w", ep, err)
	}

	rawHost := parsed.Host
	cleanKey := strings.TrimLeft(key, "/")

	// Check if path style is forced (via config, IP address, or localhost)
	usePathStyle := s.cfg.ForcePathStyle || isIPOrLocalhost(rawHost)

	if usePathStyle {
		host = rawHost
		basePath := strings.TrimSuffix(parsed.Path, "/")
		canonicalURI = fmt.Sprintf("%s/%s/%s", basePath, s.cfg.Bucket, cleanKey)
		if !strings.HasPrefix(canonicalURI, "/") {
			canonicalURI = "/" + canonicalURI
		}
		targetURL = fmt.Sprintf("%s://%s%s", parsed.Scheme, host, canonicalURI)
		return targetURL, host, canonicalURI, nil
	}

	// Virtual-hosted style (standard for Railway, AWS S3, Cloudflare R2):
	// Subdomain: {bucket}.{endpoint_host}
	// Path: /{key}
	if strings.HasPrefix(strings.ToLower(rawHost), strings.ToLower(s.cfg.Bucket)+".") {
		host = rawHost
	} else {
		host = fmt.Sprintf("%s.%s", s.cfg.Bucket, rawHost)
	}

	canonicalURI = "/" + cleanKey
	targetURL = fmt.Sprintf("%s://%s%s", parsed.Scheme, host, canonicalURI)
	return targetURL, host, canonicalURI, nil
}

func (s *S3Storage) signRequest(req *http.Request, method, host, canonicalURI string, payload []byte, contentType string, now time.Time) {
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	payloadHash := sha256Hex(payload)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Host", host)

	var signedHeaders string
	var canonicalHeaders string

	if contentType != "" {
		signedHeaders = "content-type;host;x-amz-content-sha256;x-amz-date"
		canonicalHeaders = fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
			contentType, host, payloadHash, amzDate)
	} else {
		signedHeaders = "host;x-amz-content-sha256;x-amz-date"
		canonicalHeaders = fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
			host, payloadHash, amzDate)
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.cfg.Region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSignatureKey(s.cfg.SecretKey, dateStamp, s.cfg.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKey,
		credentialScope,
		signedHeaders,
		signature,
	)

	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(key, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(regionName))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
