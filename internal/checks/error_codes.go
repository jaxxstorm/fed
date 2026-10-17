package checks

import (
	"errors"
	"github.com/aws/smithy-go"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func collectorErrorCode(err error, fallback string) string {
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorCode() {
		case "ExpiredToken", "ExpiredTokenException", "RequestExpired":
			return "expired_credentials"
		case "AccessDenied", "AccessDeniedException", "UnauthorizedOperation":
			return "access_denied"
		}
	}
	if fallback == "namespace_enumeration_failed" && apierrors.IsForbidden(err) {
		return "namespace_access_denied"
	}
	return fallback
}
