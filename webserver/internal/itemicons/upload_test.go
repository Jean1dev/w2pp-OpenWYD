package itemicons

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadWritesURLsAndResumes(t *testing.T) {
	requests := 0
	var published Manifest
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/s3" {
			t.Errorf("request = %s %s, want POST /v1/s3", r.Method, r.URL.Path)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}
		if got := r.FormValue("bucket"); got != "icons" {
			t.Errorf("bucket = %q, want icons", got)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile: %v", err)
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			t.Errorf("ReadAll: %v", err)
		}
		requests++
		if header.Filename == "published-manifest.json" {
			if err := json.Unmarshal(data, &published); err != nil {
				t.Errorf("published manifest: %v", err)
			}
		}
		fmt.Fprintf(w, "%s/public/%d", server.URL, requests)
	}))
	defer server.Close()

	packDir := t.TempDir()
	manifest := validTestManifest()
	manifest.ItemToIcon[10] = 0
	manifest.ItemToIcon[11] = 1
	manifest.MappedItems = 2
	manifest.DistinctIcons = 2
	if err := writeManifest(filepath.Join(packDir, "manifest.json"), manifest); err != nil {
		t.Fatal(err)
	}
	versionDir := filepath.Join(packDir, manifest.PackVersion)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"i0000", "i0001"} {
		if err := os.WriteFile(filepath.Join(versionDir, key+".png"), []byte(key), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	cfg := UploadConfig{
		Endpoint: server.URL, Bucket: "icons", PackDir: packDir,
		AuditPath: auditPath, Client: server.Client(),
	}
	audit, err := Upload(cfg)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if requests != 3 || audit.UploadedCount != 2 || audit.ManifestURL == "" {
		t.Fatalf("requests/audit = %d/%+v, want 3 uploads and 2 icons", requests, audit)
	}
	if len(published.IconURLs) != 2 || published.IconURLs["i0000"] == "" || published.IconURLs["i0001"] == "" {
		t.Fatalf("published URLs = %+v", published.IconURLs)
	}
	loaded, err := Load(filepath.Join(packDir, "published-manifest.json"))
	if err != nil || loaded.IconURL(10) == "" {
		t.Fatalf("published manifest load/icon URL = %+v/%v", loaded, err)
	}

	// A resumed run reuses both content hashes/URLs and only republishes the
	// small manifest; it must not duplicate the icon objects.
	if _, err := Upload(cfg); err != nil {
		t.Fatalf("resume Upload: %v", err)
	}
	if requests != 4 {
		t.Fatalf("requests after resume = %d, want 4", requests)
	}
}

func TestUploadRejectsHTTPEndpoint(t *testing.T) {
	if _, err := Upload(UploadConfig{Endpoint: "http://example.com", Bucket: "x", PackDir: "x", AuditPath: "x"}); err == nil {
		t.Fatal("HTTP endpoint accepted")
	}
}
