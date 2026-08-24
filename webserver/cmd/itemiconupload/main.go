// Command itemiconupload publishes a generated pack through
// storage-manager-server and writes a resumable URL audit.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemicons"
)

func main() {
	endpoint := flag.String("endpoint", os.Getenv("W2PP_STORAGE_MANAGER_URL"), "storage-manager-server base URL")
	bucket := flag.String("bucket", envOr("W2PP_STORAGE_MANAGER_BUCKET", "jeanluca-teste"), "S3 bucket managed by storage-manager-server")
	token := flag.String("token", os.Getenv("W2PP_STORAGE_MANAGER_TOKEN"), "optional bearer token")
	packDir := flag.String("pack", "dist/item-icons", "generated item icon pack")
	auditPath := flag.String("audit", "", "audit JSON path (default docs/audits/item-icons-upload-<version>.json)")
	flag.Parse()

	manifest, err := itemicons.Load(filepath.Join(*packDir, "manifest.json"))
	if err != nil {
		log.Fatal(err)
	}
	if *auditPath == "" {
		*auditPath = filepath.Join("docs", "audits", "item-icons-upload-"+manifest.PackVersion+".json")
	}
	audit, err := itemicons.Upload(itemicons.UploadConfig{
		Endpoint: *endpoint, Bucket: *bucket, Token: *token, PackDir: *packDir, AuditPath: *auditPath,
	})
	if err != nil {
		log.Fatal(err)
	}
	summary, err := json.Marshal(struct {
		PackVersion string `json:"pack_version"`
		Uploaded    int    `json:"uploaded"`
		ManifestURL string `json:"manifest_url"`
		AuditPath   string `json:"audit_path"`
	}{audit.PackVersion, audit.UploadedCount, audit.ManifestURL, *auditPath})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(summary))
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
