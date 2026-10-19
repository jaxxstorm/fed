// guidance-package emits authored source-pinned drafts, never review receipts.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jaxxstorm/fed/internal/catalog"
	"github.com/jaxxstorm/fed/internal/guidancecontent"
	"github.com/jaxxstorm/fed/internal/readiness"
)

func main() {
	batch := flag.String("batch", "CED", "Authored domain batch")
	hashes := flag.Bool("review-hashes", false, "Emit exact content hashes for recording separate source comparisons")
	coverage := flag.Bool("coverage", false, "Report local draft coverage and exact remaining variant IDs across all authored batches")
	comparisons := flag.String("comparison-inputs", "", "With --batch all, validate and rebind the comparison JSON files from this directory; emits inputs, not authenticated receipts")
	rules := flag.String("rules", "testdata/fedramp", "Pinned rules directory")
	markdown := flag.String("markdown", "2026-markdown", "Pinned Markdown directory")
	flag.Parse()
	if *coverage {
		if err := reportCoverage(*rules, *markdown); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	var p readiness.Package
	var err error
	switch *batch {
	case "all":
		var batches []readiness.Package
		batches, err = guidancecontent.DraftBatches(os.DirFS(*rules), os.DirFS(*markdown))
		if err == nil {
			p, err = guidancecontent.Assemble(batches, 1)
		}
	default:
		p, err = guidancecontent.BuildBatch(*batch, os.DirFS(*rules), os.DirFS(*markdown))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if *comparisons != "" {
		if *batch != "all" {
			fmt.Fprintln(os.Stderr, "comparison-inputs requires --batch all")
			os.Exit(1)
		}
		batches, e := guidancecontent.DraftBatches(os.DirFS(*rules), os.DirFS(*markdown))
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		files, e := filepath.Glob(filepath.Join(*comparisons, "*.json"))
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		bundles := []guidancecontent.ComparisonBundle{}
		for _, path := range files {
			raw, e := os.ReadFile(path)
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
			var bundle guidancecontent.ComparisonBundle
			if e = json.Unmarshal(raw, &bundle); e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
			bundles = append(bundles, bundle)
		}
		out, e := guidancecontent.RebindBundles(batches, p, bundles)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		if e = encoder.Encode(out); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	if *hashes {
		out := map[string]string{}
		for _, book := range p.Playbooks {
			hash, e := readiness.ReviewHash(p, book.ID)
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
			out[book.ID] = hash
		}
		if err = encoder.Encode(out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err = encoder.Encode(p); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func reportCoverage(rules, markdown string) error {
	packages, err := guidancecontent.DraftBatches(os.DirFS(rules), os.DirFS(markdown))
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(rules + "/rules.json")
	if err != nil {
		return err
	}
	schema, err := os.ReadFile(rules + "/schema.json")
	if err != nil {
		return err
	}
	snapshot, err := catalog.Parse(raw, schema, catalog.PinnedCommit)
	if err != nil {
		return err
	}
	inventory, err := catalog.Inventory(snapshot)
	if err != nil {
		return err
	}
	report, err := guidancecontent.Coverage(packages, inventory)
	if err != nil {
		return err
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(report)
}
