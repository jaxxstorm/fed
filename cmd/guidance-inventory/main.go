// guidance-inventory emits a deterministic authoring manifest without database
// access or activation of any guidance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/jaxxstorm/fed/internal/catalog"
)

func main() {
	payloadPath := flag.String("rules", "testdata/fedramp/rules.json", "Pinned rules JSON")
	schemaPath := flag.String("schema", "testdata/fedramp/schema.json", "Pinned rules schema")
	commit := flag.String("commit", catalog.PinnedCommit, "Rules commit")
	manifest := flag.Bool("manifest", false, "Emit the compact per-domain authoring manifest")
	flag.Parse()
	if err := run(*payloadPath, *schemaPath, *commit, *manifest); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(payloadPath, schemaPath, commit string, manifest bool) error {
	payload, err := os.ReadFile(payloadPath)
	if err != nil {
		return err
	}
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	snapshot, err := catalog.Parse(payload, schema, commit)
	if err != nil {
		return err
	}
	inventory, err := catalog.Inventory(snapshot)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	if manifest {
		return encoder.Encode(catalog.Authoring(inventory))
	}
	return encoder.Encode(inventory)
}
