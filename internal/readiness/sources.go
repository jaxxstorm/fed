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
	return newSourceVerifier(rules, markdown).verify(p)
}

// A verifier belongs to one operation, never to a Store or a global cache.
// Read/hash each corpus file once in that operation; the next request rereads
// the filesystem so changed or unavailable sources still invalidate guidance.
type sourceVerifier struct {
	rules, markdown fs.FS
	files           map[string]verifiedSourceFile
	citations       map[Citation]error
}

type verifiedSourceFile struct {
	content []byte
	hash    string
	err     error
}

func newSourceVerifier(rules, markdown fs.FS) *sourceVerifier {
	return &sourceVerifier{rules: rules, markdown: markdown, files: map[string]verifiedSourceFile{}, citations: map[Citation]error{}}
}

func (v *sourceVerifier) file(source, path string) verifiedSourceFile {
	key := source + ":" + path
	if f, ok := v.files[key]; ok {
		return f
	}
	root := v.rules
	if source == "site" {
		root = v.markdown
	}
	content, err := fs.ReadFile(root, path)
	f := verifiedSourceFile{content: content, hash: storage.Hash(content), err: err}
	v.files[key] = f
	return f
}

func (v *sourceVerifier) verify(p Package) error {
	rules, markdown := v.rules, v.markdown
	if rules == nil || markdown == nil {
		return fmt.Errorf("official source corpus unavailable")
	}
	metadata := v.file("site", "_sources.json")
	if metadata.err != nil {
		return metadata.err
	}
	var upstream struct {
		Site  struct{ Repo, SHA string }
		Rules struct{ Repo, SHA string }
	}
	if err := json.Unmarshal(metadata.content, &upstream); err != nil {
		return err
	}
	for _, entry := range []struct{ key, repo, commit string }{{"rules", upstream.Rules.Repo, upstream.Rules.SHA}, {"site", upstream.Site.Repo, upstream.Site.SHA}} {
		s, ok := p.Sources[entry.key]
		if !ok || !commitPattern.MatchString(s.Commit) || s.Commit != entry.commit || !strings.EqualFold(s.Repository, entry.repo) {
			return fmt.Errorf("%s source does not match corpus provenance", entry.key)
		}
	}
	raw := v.file("rules", "rules.json")
	if raw.err != nil {
		return raw.err
	}
	if p.CatalogID != p.Sources["rules"].Commit+":"+raw.hash {
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
		switch c.Source {
		case "rules":
		case "site":
		default:
			return fmt.Errorf("citation %s has unsupported source", c.ID)
		}
		if err, ok := v.citations[c]; ok {
			if err != nil {
				return err
			}
			continue
		}
		content := v.file(c.Source, c.Path)
		if content.err != nil {
			return fmt.Errorf("citation %s: %w", c.ID, content.err)
		}
		var err error
		if content.hash != c.ContentHash || storage.Hash([]byte(c.Excerpt)) != c.ExcerptHash || !SourceExcerptMatches(content.content, c.Source, c.Anchor, c.Excerpt) {
			err = fmt.Errorf("citation %s needs revalidation", c.ID)
		}
		v.citations[c] = err
		if err != nil {
			return err
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
