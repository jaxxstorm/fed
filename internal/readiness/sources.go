package readiness

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"

	"github.com/jaxxstorm/fed/internal/storage"
)

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// VerifySources compares the immutable package to the checked-out corpus and
// rules. fs.FS roots are supplied by the operator, never by untrusted citations.
func VerifySources(p Package, rules, markdown fs.FS) error {
	if rules == nil || markdown == nil {
		return fmt.Errorf("official source corpus unavailable")
	}
	metadata, err := fs.ReadFile(markdown, "_sources.json")
	if err != nil {
		return err
	}
	var upstream struct {
		Site  struct{ Repo, SHA string }
		Rules struct{ Repo, SHA string }
	}
	if err = json.Unmarshal(metadata, &upstream); err != nil {
		return err
	}
	for _, entry := range []struct{ key, repo, commit string }{{"rules", upstream.Rules.Repo, upstream.Rules.SHA}, {"site", upstream.Site.Repo, upstream.Site.SHA}} {
		s, ok := p.Sources[entry.key]
		if !ok || !commitPattern.MatchString(s.Commit) || s.Commit != entry.commit || !strings.EqualFold(s.Repository, entry.repo) {
			return fmt.Errorf("%s source does not match corpus provenance", entry.key)
		}
	}
	raw, err := fs.ReadFile(rules, "rules.json")
	if err != nil {
		return err
	}
	if p.CatalogID != p.Sources["rules"].Commit+":"+storage.Hash(raw) {
		return fmt.Errorf("rules payload does not match catalog identity")
	}
	seen := map[string]bool{}
	for _, c := range p.Citations {
		if c.ID == "" || seen[c.ID] {
			return fmt.Errorf("duplicate or empty citation %q", c.ID)
		}
		seen[c.ID] = true
		if !fs.ValidPath(c.Path) || c.Excerpt == "" || c.Anchor == "" {
			return fmt.Errorf("citation %s has invalid location or excerpt", c.ID)
		}
		var root fs.FS
		switch c.Source {
		case "rules":
			root = rules
		case "site":
			root = markdown
		default:
			return fmt.Errorf("citation %s has unsupported source", c.ID)
		}
		content, err := fs.ReadFile(root, c.Path)
		if err != nil {
			return fmt.Errorf("citation %s: %w", c.ID, err)
		}
		if storage.Hash(content) != c.ContentHash || storage.Hash([]byte(c.Excerpt)) != c.ExcerptHash || !SourceExcerptMatches(content, c.Source, c.Anchor, c.Excerpt) {
			return fmt.Errorf("citation %s needs revalidation", c.ID)
		}
	}
	return nil
}

// SourceExcerptMatches also handles decoded JSON string excerpts. Official
// statements containing quotes, escaped newlines or Unicode escapes need not be
// displayed as JSON escape sequences merely to preserve their provenance.
func SourceExcerptMatches(content []byte, source, anchor, excerpt string) bool {
	if excerpt == "" {
		return false
	}
	if strings.Contains(string(content), excerpt) {
		return true
	}
	if source != "rules" || !strings.HasPrefix(anchor, "/") {
		return false
	}
	var value any
	if json.Unmarshal(content, &value) != nil {
		return false
	}
	for _, token := range strings.Split(anchor[1:], "/") {
		for i := 0; i < len(token); i++ {
			if token[i] == '~' {
				if i+1 >= len(token) || (token[i+1] != '0' && token[i+1] != '1') {
					return false
				}
				i++
			}
		}
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch current := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = current[token]
			if !ok {
				return false
			}
		case []any:
			index, e := strconv.Atoi(token)
			if e != nil || index < 0 || index >= len(current) || strconv.Itoa(index) != token {
				return false
			}
			value = current[index]
		default:
			return false
		}
	}
	text, ok := value.(string)
	return ok && strings.Contains(text, excerpt)
}
