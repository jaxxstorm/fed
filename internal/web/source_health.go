package web

import (
	"context"
	"encoding/json"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/observation"
	"time"
)

type sourceRead struct {
	Submission observation.Submission
	At         time.Time
}

func (app *App) sourceReads(ctx context.Context, service string) ([]sourceRead, error) {
	rows, err := app.Store.DB.Query(ctx, `SELECT DISTINCT ON(target_key) data,observed_at FROM observations WHERE service_id=$1 AND scope_revision=(SELECT max(revision) FROM records WHERE service_id=$1 AND kind='scope' AND id='scope') ORDER BY target_key,observed_at DESC,received_at DESC`, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []sourceRead{}
	for rows.Next() {
		var raw []byte
		var r sourceRead
		if err = rows.Scan(&raw, &r.At); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r.Submission); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func sourceHints(source assessment.EvidenceSource, reads []sourceRead) []string {
	hints := []string{source.Type + " · " + source.Locator}
	var newest *sourceRead
	failed := 0
	checked := 0
	for _, read := range reads {
		target := read.Submission.Target
		match := false
		switch source.Type {
		case "aws":
			match = target.AccountID == source.AccountID && target.Region == source.Region
		case "kubernetes":
			if source.ClusterID != "" {
				match = target.ClusterID == source.ClusterID && (source.NamespaceUID == "" || target.NamespaceUID == source.NamespaceUID)
			} else {
				match = target.AccountID == source.AccountID && target.Region == source.Region && target.Cluster == source.Cluster
			}
		case "repository":
			match = target.Repository == source.Repository && target.Commit == source.Commit
		}
		if !match {
			continue
		}
		checked++
		if read.Submission.Coverage.Error != "" {
			failed++
		}
		if newest == nil || read.At.After(newest.At) {
			copy := read
			newest = &copy
		}
	}
	if newest == nil {
		hints = append(hints, "No collection results recorded for this source in the current assessment scope.", "This does not mean access was denied. Connection tests and discovery are tracked separately in Connections; neither gathers assessment evidence.", "Next: open Connections and test this source’s provider access, then run supported collection from Activity or ask an external agent to submit observations for this source and current scope.")
	} else {
		state := "Latest recorded reads succeeded"
		if failed > 0 {
			state = "Some target reads could not finish — inspect collection activity"
		}
		hints = append(hints, state+" · "+newest.At.UTC().Format("02 Jan 2006 15:04 UTC"))
	}
	if source.AccessHint != "" {
		hints = append(hints, "Agent access hint: "+source.AccessHint)
	}
	return hints
}
