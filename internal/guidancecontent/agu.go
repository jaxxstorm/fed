package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

// AGU retains all 20 placeholder-source decisions as agency-owned questions.
func AGU(rules, markdown fs.FS) (readiness.Package, error) {
	return administrativeDraft("AGU", rules, markdown, nil)
}
