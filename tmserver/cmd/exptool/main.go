// Command exptool restamps the monster templates' kill-reward Exp field
// (STRUCT_MOB.Exp @32) in a Release npc directory with the balanced level
// curve — see tmserver/internal/exptool and issue #43. Run with -dry-run to
// review the report, then without it and commit the regenerated binaries.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/exptool"
)

func main() {
	npc := flag.String("npc", "Release/TMsrv/run/npc", "directory with the raw 816-byte STRUCT_MOB templates")
	dryRun := flag.Bool("dry-run", false, "report what would change without writing")
	quiet := flag.Bool("quiet", false, "only print the summary line")
	flag.Parse()

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
	fmt.Printf("%s %d monsters (%d changed), skipped %d non-monster/invalid files in %s\n",
		mode, len(res.Stamped), changed, res.Skipped, *npc)
}
