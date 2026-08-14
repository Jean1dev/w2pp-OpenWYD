// Command exptool restamps the monster templates' kill-reward Exp field
// (STRUCT_MOB.Exp) in a Release npc directory with the balanced level curve —
// see tmserver/internal/exptool and issue #43. Every template layout in that
// directory is covered, including the legacy 756/920-byte ones, and no file
// changes size. Run with -dry-run to review the report, then without it and
// commit the regenerated binaries.
//
// With -inventory it instead censuses the directory by on-disk STRUCT_MOB
// layout and writes nothing — the reproducible audit issue #244 asks for.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jeanluca/w2pp-openwyd/internal/npctemplate"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/exptool"
)

func main() {
	npc := flag.String("npc", "Release/TMsrv/run/npc", "directory with the raw STRUCT_MOB templates")
	dryRun := flag.Bool("dry-run", false, "report what would change without writing")
	quiet := flag.Bool("quiet", false, "only print the summary line")
	inventory := flag.Bool("inventory", false, "census the directory by STRUCT_MOB layout and exit (never writes)")
	sample := flag.Int("inventory-sample", 0, "with -inventory, cap how many names are listed per layout (0 = all)")
	flag.Parse()

	if *inventory {
		if err := runInventory(*npc, *sample); err != nil {
			fmt.Fprintln(os.Stderr, "exptool:", err)
			os.Exit(1)
		}
		return
	}

	res, err := exptool.Stamp(*npc, *dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, "exptool:", err)
		os.Exit(1)
	}
	changed := 0
	for _, e := range res.Stamped {
		if e.OldExp != e.NewExp {
			changed++
		}
		if !*quiet {
			fmt.Printf("%-16s lvl %3d  exp %d -> %d\n", e.File, e.Level, e.OldExp, e.NewExp)
		}
	}
	mode := "stamped"
	if *dryRun {
		mode = "would stamp"
	}
	fmt.Printf("%s %d monsters (%d changed, %d in a legacy layout), skipped %d non-monster and %d non-template files in %s\n",
		mode, len(res.Stamped), changed, res.StampedVariant, res.Skipped, res.SkippedNonTemplate, *npc)
}

func runInventory(dir string, sample int) error {
	inv, err := npctemplate.TakeInventory(dir)
	if err != nil {
		return err
	}
	for _, line := range inv.Report(sample) {
		fmt.Println(line)
	}
	return nil
}
