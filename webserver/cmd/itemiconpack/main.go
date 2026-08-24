// Command itemiconpack extracts the classic client item icons into a
// versioned, CDN-ready directory without adding proprietary pixels to Git.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemcatalog"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemicons"
)

func main() {
	clientDir := flag.String("client", "", "path containing itemicon.bin and UI/itemiconNN.wyt")
	outputDir := flag.String("out", "dist/item-icons", "output directory")
	contentDir := flag.String("content", "Release", "content tree used to report named-item coverage (empty = skip)")
	flag.Parse()
	if *clientDir == "" {
		log.Fatal("itemiconpack: -client is required")
	}
	manifest, err := itemicons.Generate(*clientDir, *outputDir)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("pack_version=%s mapped_items=%d distinct_icons=%d atlases=%d\n", manifest.PackVersion, manifest.MappedItems, manifest.DistinctIcons, len(manifest.Atlases))
	if *contentDir != "" {
		catalog, err := itemcatalog.Scan(*contentDir)
		if err != nil {
			log.Fatalf("itemiconpack: scan coverage catalog: %v", err)
		}
		mapped := 0
		for _, item := range catalog.Items {
			if manifest.IconKey(item.Index) != "" {
				mapped++
			}
		}
		coverage := 0.0
		if len(catalog.Items) != 0 {
			coverage = float64(mapped) * 100 / float64(len(catalog.Items))
		}
		fmt.Printf("catalog_items=%d mapped_catalog_items=%d coverage=%.2f%%\n", len(catalog.Items), mapped, coverage)
	}
}
