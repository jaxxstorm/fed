// Package integrations defines optional vendor-neutral integration contracts.
package integrations

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("integration_unavailable")

type Capabilities struct {
	ListEvidence     bool `json:"list_evidence"`
	RetrieveEvidence bool `json:"retrieve_evidence"`
	Publish          bool `json:"publish"`
}
type ExternalReference struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Description string `json:"description"`
}
type Receipt struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}
type TrustCentre interface {
	Capabilities() Capabilities
	ListEvidence(context.Context, string, string) ([]ExternalReference, string, error)
	RetrieveEvidence(context.Context, string, string) ([]byte, error)
	Publish(context.Context, string, string, []byte) (Receipt, error)
}
type Unconfigured struct{}

func (Unconfigured) Capabilities() Capabilities { return Capabilities{} }
func (Unconfigured) ListEvidence(context.Context, string, string) ([]ExternalReference, string, error) {
	return nil, "", ErrUnavailable
}
func (Unconfigured) RetrieveEvidence(context.Context, string, string) ([]byte, error) {
	return nil, ErrUnavailable
}
func (Unconfigured) Publish(context.Context, string, string, []byte) (Receipt, error) {
	return Receipt{}, ErrUnavailable
}
