package itemicons

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// UploadConfig describes one resumable publication through
// storage-manager-server's multipart S3 endpoint.
type UploadConfig struct {
	Endpoint  string
	Bucket    string
	Token     string
	PackDir   string
	AuditPath string
	// Client is optional and primarily permits a custom TLS trust store in
	// tests or private deployments. Nil uses a 60-second default client.
	Client *http.Client
}

// UploadAudit records every public URL returned by storage-manager-server.
// It is safe to commit: credentials and request headers are never included.
type UploadAudit struct {
	Version       int                `json:"version"`
	Endpoint      string             `json:"endpoint"`
	Bucket        string             `json:"bucket"`
	PackVersion   string             `json:"pack_version"`
	StartedAt     string             `json:"started_at"`
	CompletedAt   string             `json:"completed_at,omitempty"`
	ManifestURL   string             `json:"manifest_url,omitempty"`
	UploadedCount int                `json:"uploaded_count"`
	Entries       []UploadAuditEntry `json:"entries"`
}

// UploadAuditEntry records one local icon, its content hash and returned URL.
type UploadAuditEntry struct {
	IconKey    string `json:"icon_key"`
	SHA256     string `json:"sha256"`
	URL        string `json:"url,omitempty"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	UploadedAt string `json:"uploaded_at,omitempty"`
}

// Upload publishes every distinct icon, writes progress after each request and
// finally uploads a published manifest containing the returned URL map.
func Upload(cfg UploadConfig) (UploadAudit, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.PackDir == "" || cfg.AuditPath == "" {
		return UploadAudit{}, fmt.Errorf("itemicons: endpoint, bucket, pack dir and audit path are required")
	}
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	if u, err := url.Parse(endpoint); err != nil || u.Scheme != "https" || u.Host == "" {
		return UploadAudit{}, fmt.Errorf("itemicons: endpoint must be an absolute HTTPS URL")
	}
	manifestPath := filepath.Join(cfg.PackDir, "manifest.json")
	manifest, err := Load(manifestPath)
	if err != nil {
		return UploadAudit{}, err
	}
	if manifest.IconURLs == nil {
		manifest.IconURLs = make(map[string]string, manifest.DistinctIcons)
	}

	audit, err := loadOrCreateAudit(cfg, manifest.PackVersion)
	if err != nil {
		return UploadAudit{}, err
	}
	icons := distinctIconKeys(manifest)
	entries := make(map[string]UploadAuditEntry, len(audit.Entries))
	for _, entry := range audit.Entries {
		entries[entry.IconKey] = entry
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	for _, key := range icons {
		path := filepath.Join(cfg.PackDir, manifest.PackVersion, key+".png")
		data, err := os.ReadFile(path)
		if err != nil {
			return audit, fmt.Errorf("itemicons: read %s: %w", path, err)
		}
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		if prior, ok := entries[key]; ok && prior.Status == "uploaded" && prior.SHA256 == digest && prior.URL != "" {
			manifest.IconURLs[key] = prior.URL
			continue
		}
		entry := UploadAuditEntry{IconKey: key, SHA256: digest, Status: "uploading"}
		entries[key] = entry
		audit.Entries = orderedAuditEntries(entries)
		if err := writeAudit(cfg.AuditPath, audit); err != nil {
			return audit, err
		}
		publicURL, err := uploadMultipart(client, endpoint+"/v1/s3", cfg.Bucket, cfg.Token, key+".png", data)
		if err != nil {
			entry.Status = "failed"
			entry.Error = err.Error()
			entries[key] = entry
			audit.Entries = orderedAuditEntries(entries)
			_ = writeAudit(cfg.AuditPath, audit)
			return audit, fmt.Errorf("itemicons: upload %s: %w", key, err)
		}
		entry.Status = "uploaded"
		entry.URL = publicURL
		entry.UploadedAt = time.Now().UTC().Format(time.RFC3339)
		entries[key] = entry
		manifest.IconURLs[key] = publicURL
		audit.UploadedCount = countUploaded(entries)
		audit.Entries = orderedAuditEntries(entries)
		if err := writeAudit(cfg.AuditPath, audit); err != nil {
			return audit, err
		}
	}

	publishedPath := filepath.Join(cfg.PackDir, "published-manifest.json")
	if err := writeManifest(publishedPath, manifest); err != nil {
		return audit, err
	}
	published, err := os.ReadFile(publishedPath)
	if err != nil {
		return audit, fmt.Errorf("itemicons: read published manifest: %w", err)
	}
	manifestURL, err := uploadMultipart(client, endpoint+"/v1/s3", cfg.Bucket, cfg.Token, "published-manifest.json", published)
	if err != nil {
		return audit, fmt.Errorf("itemicons: upload published manifest: %w", err)
	}
	audit.ManifestURL = manifestURL
	audit.UploadedCount = countUploaded(entries)
	audit.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	audit.Entries = orderedAuditEntries(entries)
	if err := writeAudit(cfg.AuditPath, audit); err != nil {
		return audit, err
	}
	return audit, nil
}

func loadOrCreateAudit(cfg UploadConfig, packVersion string) (UploadAudit, error) {
	b, err := os.ReadFile(cfg.AuditPath)
	if err == nil {
		var audit UploadAudit
		if err := json.Unmarshal(b, &audit); err != nil {
			return UploadAudit{}, fmt.Errorf("itemicons: decode audit %s: %w", cfg.AuditPath, err)
		}
		if audit.Version != 1 || audit.PackVersion != packVersion || audit.Endpoint != strings.TrimRight(cfg.Endpoint, "/") || audit.Bucket != cfg.Bucket {
			return UploadAudit{}, fmt.Errorf("itemicons: audit %s belongs to another upload", cfg.AuditPath)
		}
		return audit, nil
	}
	if !os.IsNotExist(err) {
		return UploadAudit{}, fmt.Errorf("itemicons: read audit %s: %w", cfg.AuditPath, err)
	}
	return UploadAudit{
		Version: 1, Endpoint: strings.TrimRight(cfg.Endpoint, "/"), Bucket: cfg.Bucket,
		PackVersion: packVersion, StartedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func uploadMultipart(client *http.Client, endpoint, bucket, token, filename string, data []byte) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("create multipart file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("write multipart file: %w", err)
	}
	if err := w.WriteField("bucket", bucket); err != nil {
		return "", fmt.Errorf("write bucket field: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close multipart: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	publicURL := strings.Trim(strings.TrimSpace(string(responseBody)), "\"")
	u, err := url.Parse(publicURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("invalid public URL %q", publicURL)
	}
	return publicURL, nil
}

func distinctIconKeys(manifest Manifest) []string {
	seen := make(map[int]struct{}, manifest.DistinctIcons)
	for _, icon := range manifest.ItemToIcon {
		if icon >= 0 {
			seen[icon] = struct{}{}
		}
	}
	ids := make([]int, 0, len(seen))
	for icon := range seen {
		ids = append(ids, icon)
	}
	sort.Ints(ids)
	keys := make([]string, len(ids))
	for i, icon := range ids {
		keys[i] = fmt.Sprintf("i%04d", icon)
	}
	return keys
}

func orderedAuditEntries(entries map[string]UploadAuditEntry) []UploadAuditEntry {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]UploadAuditEntry, 0, len(keys))
	for _, key := range keys {
		out = append(out, entries[key])
	}
	return out
}

func countUploaded(entries map[string]UploadAuditEntry) int {
	count := 0
	for _, entry := range entries {
		if entry.Status == "uploaded" {
			count++
		}
	}
	return count
}

func writeManifest(path string, manifest Manifest) error {
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("itemicons: published manifest: %w", err)
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("itemicons: encode published manifest: %w", err)
	}
	return writeAtomic(path, append(b, '\n'))
}

func writeAudit(path string, audit UploadAudit) error {
	b, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		return fmt.Errorf("itemicons: encode audit: %w", err)
	}
	return writeAtomic(path, append(b, '\n'))
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("itemicons: create %s: %w", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".itemicons-*")
	if err != nil {
		return fmt.Errorf("itemicons: create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("itemicons: write temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("itemicons: close temporary file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("itemicons: replace %s: %w", path, err)
	}
	return nil
}
