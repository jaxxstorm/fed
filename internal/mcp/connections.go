package mcp

import (
	"context"

	"github.com/jaxxstorm/fed/internal/auth"
	"github.com/jaxxstorm/fed/internal/connections"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type ConnectionOptions struct {
	Service      *connections.Service
	PublicOrigin string
}

type ProviderInput struct {
	ServiceID string               `json:"service_id"`
	Provider  connections.Provider `json:"provider"`
}

type ConnectionListInput struct {
	ServiceID      string `json:"service_id"`
	IncludeDeleted bool   `json:"include_deleted"`
}

type DiscoveryStartInput struct {
	connections.OperationInput
	Execution string `json:"execution" jsonschema:"server or agent; agent inventory is explicitly attributed and does not imply verified server access"`
}

func registerConnections(server *sdk.Server, a auth.Actor, options ConnectionOptions) {
	s := options.Service
	add(server, "rotate_aws_setup_external_id", "Explicitly replace the setup external ID, fence existing connection access and require updated customer trust and retesting.", func(ctx context.Context, in connections.SetupOperationInput) (any, error) {
		return s.RotateAWSSetupExternalID(ctx, a, in)
	})
	add(server, "get_provider_setup_readiness", "Check operator registration, permissions and runtime readiness without exposing secrets.", func(ctx context.Context, in ProviderInput) (any, error) {
		return s.ProviderSetupReadiness(ctx, a, in.ServiceID, in.Provider)
	})
	add(server, "begin_provider_setup", "Save a temporary provider setup before consent. Does not create sources or evidence.", func(ctx context.Context, in connections.BeginSetupInput) (any, error) {
		return s.BeginSetup(ctx, a, in)
	})
	add(server, "get_provider_setup", "Read redacted setup progress and verified identity.", func(ctx context.Context, in GetInput) (any, error) { return s.GetSetup(ctx, a, in.ServiceID, in.ID) })
	add(server, "update_provider_setup", "Save declarations with expected revision; edits invalidate earlier consent attempts.", func(ctx context.Context, in connections.UpdateSetupInput) (any, error) {
		return s.UpdateSetup(ctx, a, in.SetupOperationInput, in.Input)
	})
	add(server, "cancel_provider_setup", "Cancel setup and remove staged grants, retaining existing evidence.", func(ctx context.Context, in connections.SetupOperationInput) (any, error) {
		return s.CancelSetup(ctx, a, in)
	})
	add(server, "complete_provider_setup", "Confirm the provider-verified identity and atomically activate one connection. Scope selection remains explicit.", func(ctx context.Context, in connections.SetupOperationInput) (any, error) {
		return s.CompleteSetup(ctx, a, in)
	})
	add(server, "get_aws_setup_material", "Retrieve generated customer trust/read policies and CLI instructions. Requires connect permission; material contains a connection-specific external ID.", func(ctx context.Context, in connections.SetupOperationInput) (any, error) {
		return s.AWSSetupMaterial(ctx, a, in)
	})
	add(server, "verify_aws_setup", "Verify customer role delegation using Fed runtime credentials, never agent credentials.", func(ctx context.Context, in connections.SetupOperationInput) (any, error) {
		return s.VerifyAWSSetup(ctx, a, in)
	})
	add(server, "authorize_provider_setup", "Create a short-lived same-origin handoff for an authorized human browser to grant provider consent.", func(ctx context.Context, in connections.SetupOperationInput) (any, error) {
		return s.SetupHandoff(ctx, a, in, options.PublicOrigin)
	})
	add(server, "resolve_connection_repository_commit", "Resolve a discovered repository branch or ref to a full immutable commit using the selected installation. Select evidence paths only after resolving the commit.", func(ctx context.Context, in connections.ResolveCommitInput) (any, error) {
		return s.ResolveRepositoryCommit(ctx, a, in)
	})
	add(server, "refresh_evidence_source", "Collect a new immutable snapshot from a selected active source. Requires check permission and current source, scope and credential generations. Discovery and renewal do not refresh evidence.", func(ctx context.Context, in connections.RefreshSourceInput) (any, error) {
		return s.RefreshSource(ctx, a, in)
	})
	add(server, "list_connection_providers", "List provider capabilities and versioned setup guides. Application registration and provider access are separate from evidence completion.", func(ctx context.Context, in ServiceInput) (any, error) {
		if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "read"); err != nil {
			return nil, err
		}
		return s.Registry.Providers(), nil
	})
	add(server, "get_connection_setup_instructions", "Read registration prerequisites, execution environment, read permissions, callback path and recovery instructions for a provider.", func(ctx context.Context, in ProviderInput) (any, error) {
		if err := auth.Authorize(ctx, s.DB, a, in.ServiceID, "read"); err != nil {
			return nil, err
		}
		provider, ok := s.Registry.Provider(in.Provider)
		if !ok {
			return nil, connections.ErrInvalid
		}
		return struct {
			Provider connections.ProviderDefinition `json:"provider"`
			Recovery []connections.Recovery         `json:"recovery"`
		}{provider, connections.RecoveryCatalog()}, nil
	})
	add(server, "list_evidence_connections", "List non-secret connection metadata and attributed access health. Retired connections require include_deleted.", func(ctx context.Context, in ConnectionListInput) (any, error) {
		return s.List(ctx, a, in.ServiceID, in.IncludeDeleted)
	})
	add(server, "get_evidence_connection", "Read a connection's configuration, revision, credential generation and access health without retrieving credentials.", func(ctx context.Context, in GetInput) (any, error) { return s.Get(ctx, a, in.ServiceID, in.ID) })
	add(server, "configure_evidence_connection", "Create or configure a connection with connect permission, expected revision and idempotency key. Configuration does not prove access.", func(ctx context.Context, in connections.ConfigureInput) (any, error) { return s.Configure(ctx, a, in) })
	add(server, "test_evidence_connection", "Test server access and return a durable operation receipt. Agent-managed access must be reported through discovery using the agent's own tools.", func(ctx context.Context, in connections.OperationInput) (any, error) {
		return s.TestWithReceipt(ctx, a, in)
	})
	add(server, "renew_evidence_connection", "Renew provider access or return environment-specific instructions. Renewal never extends existing evidence freshness.", func(ctx context.Context, in connections.OperationInput) (any, error) { return s.Renew(ctx, a, in) })
	add(server, "get_connection_operation", "Read a durable operation outcome. Authorization codes, credentials and PKCE verifiers are never returned.", func(ctx context.Context, in GetInput) (any, error) {
		return s.GetOperation(ctx, a, in.ServiceID, in.ID)
	})
	add(server, "begin_connection_authorization", "Start a short-lived same-origin handoff for an authorized browser user to complete provider consent. Poll get_connection_operation before continuing.", func(ctx context.Context, in connections.OperationInput) (any, error) {
		return s.BeginAuthorization(ctx, a, in, options.PublicOrigin)
	})
	for _, operation := range []string{"disconnect", "delete", "restore"} {
		add(server, operation+"_evidence_connection", "Change local connection lifecycle with expected revision and generation. History is retained; provider resources are never deleted.", func(ctx context.Context, in connections.OperationInput) (any, error) {
			return s.ChangeLifecycle(ctx, a, in, operation)
		})
	}
	add(server, "discover_connection_sources", "Start a resumable discovery session. Use continue_connection_discovery for server reads or submit_connection_discovery with your own observed inventory.", func(ctx context.Context, in DiscoveryStartInput) (any, error) {
		switch in.Execution {
		case "server":
			return s.StartServerDiscovery(ctx, a, in.OperationInput)
		case "agent":
			return s.StartAgentDiscovery(ctx, a, in.OperationInput)
		default:
			return nil, connections.ErrInvalid
		}
	})
	add(server, "continue_connection_discovery", "Read the next bounded server discovery page using the session cursor and a unique page idempotency key.", func(ctx context.Context, in connections.ServerDiscoveryInput) (any, error) {
		return s.DiscoverServerPage(ctx, a, in)
	})
	add(server, "submit_connection_discovery", "Submit an attributed page discovered with your own tools. Supply stable IDs, observation time, method and limitations. Inventory does not select assessment sources.", func(ctx context.Context, in connections.DiscoverySubmission) (any, error) {
		return s.SubmitAgentDiscovery(ctx, a, in)
	})
	add(server, "list_connection_sources", "Search paginated inventory with selection filters and opaque cursors. Existing source bindings identify resources already selected through another connection.", func(ctx context.Context, in connections.InventoryQuery) (any, error) {
		return s.ListInventory(ctx, a, in)
	})
	add(server, "select_connection_sources", "Atomically select resources and explicitly requested scope additions. Requires connect/edit, scope revision and pinned repository commits/paths.", func(ctx context.Context, in connections.SelectionInput) (any, error) {
		return s.SelectSources(ctx, a, in)
	})
	add(server, "configure_evidence_source", "Create or configure a source under an active connection within existing scope. Binding identity cannot be silently changed.", func(ctx context.Context, in connections.SourceConfigurationInput) (any, error) {
		return s.ConfigureSource(ctx, a, in)
	})
	for name, operation := range map[string]string{"delete_evidence_source": "retire", "disable_evidence_source": "disable", "restore_evidence_source": "restore", "rebind_evidence_source": "rebind"} {
		add(server, name, "Explicitly change source lifecycle or credential binding. Retained evidence and exact historical citations remain readable.", func(ctx context.Context, in connections.SourceOperationInput) (any, error) {
			return s.ChangeSource(ctx, a, in, operation)
		})
	}
}
