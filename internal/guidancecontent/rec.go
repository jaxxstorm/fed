package guidancecontent

import (
	"github.com/jaxxstorm/fed/internal/readiness"
	"io/fs"
)

// REC preserves assessor/FedRAMP responsibilities excluded by the provider inventory.
func REC(rules, markdown fs.FS) (readiness.Package, error) {
	return administrativeDraft("REC", rules, markdown, nil)
}
