package checks

import (
	"errors"
	"github.com/aws/smithy-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestTypedCollectorErrors(t *testing.T) {
	for _, tc := range []struct {
		err            error
		fallback, want string
	}{{&smithy.GenericAPIError{Code: "ExpiredToken"}, "read_failed", "expired_credentials"}, {apierrors.NewForbidden(schema.GroupResource{Resource: "namespaces"}, "", errors.New("denied")), "namespace_enumeration_failed", "namespace_access_denied"}, {errors.New("network timeout"), "read_failed", "read_failed"}} {
		if got := collectorErrorCode(tc.err, tc.fallback); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}
