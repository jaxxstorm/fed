import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { api, date, human, request } from "./api";
import { ErrorPanel } from "./forms";
import type { components } from "./generated";
import { ProviderSetup } from "./provider-setup";

type Schema = components["schemas"];
type Detail = Schema["Detail"];
type Provider = Schema["ProviderDefinition"];
type Resource = Schema["InventoryResource"];
function newID() {
  const bytes = globalThis.crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 15) | 64;
  bytes[8] = (bytes[8] & 63) | 128;
  const hex = Array.from(bytes, (byte) =>
    byte.toString(16).padStart(2, "0"),
  ).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
const resourceKey = (resource: Resource) =>
  `${resource.resource.kind}/${resource.resource.id}`;

export function ConnectionsPage({
  service,
  csrf,
  capabilities,
}: {
  service: string;
  csrf: string;
  capabilities: string[];
}) {
  const [params, setParams] = useSearchParams();
  const cache = useQueryClient();
  const [provider, setProvider] = useState(params.get("provider") || "");
  const [advanced, setAdvanced] = useState(false);
  const standard = ["aws", "github", "google_workspace"].includes(provider);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [notice, setNotice] = useState("");
  const base = `${api(service)}/connections`;
  const operationID = params.get("operation");
  const resumed = useQuery({
    queryKey: [service, "connection-operation", operationID],
    enabled: !!operationID,
    queryFn: () =>
      request<Schema["Operation"]>(
        `${base}/operations/${encodeURIComponent(operationID || "")}/status`,
      ),
    refetchInterval: (query) =>
      ["pending", "running"].includes(query.state.data?.state || "")
        ? 2000
        : false,
  });
  const catalog = useQuery({
    queryKey: [service, "connection-providers"],
    queryFn: () => request<Provider[]>(`${base}/providers`),
  });
  const list = useQuery({
    queryKey: [service, "connections"],
    queryFn: () => request<Detail[]>(`${base}?include_deleted=true`),
  });
  const selected = list.data?.find(
    (item) => item.connection.id === params.get("connection"),
  );
  const guide = catalog.data?.find(
    (item) => item.id === (selected?.connection.provider || provider),
  );
  const canConnect = capabilities.includes("connect");
  const scope = useQuery({
    queryKey: [service, "connection-scope"],
    queryFn: () => request<Schema["Record"]>(`${base}/scope`),
  });
  async function post<T>(action: string, body: object): Promise<T> {
    return request<T>(`${base}/${action}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ ...body, service_id: service }),
    });
  }
  async function run(action: () => Promise<void>) {
    setPending(true);
    setError(null);
    setNotice("");
    try {
      await action();
      await cache.invalidateQueries({ queryKey: [service] });
    } catch (cause) {
      setError(cause instanceof Error ? cause : new Error("Operation failed"));
    } finally {
      setPending(false);
    }
  }
  function input() {
    if (!selected) throw new Error("Select a connection");
    return {
      connection_id: selected.connection.id,
      expected_revision: selected.connection.revision,
      generation: selected.connection.generation,
      idempotency_key: newID(),
    };
  }
  async function operation(action: string) {
    await run(async () => {
      if (action === "authorize") {
        if (
          selected?.connection.access_strategy &&
          selected.connection.access_strategy !== "legacy"
        ) {
          const connection = selected.connection;
          const setup = await post<Schema["Setup"]>("setups/begin", {
            ...input(),
            provider: connection.provider,
            input: {
              name: connection.name,
              ...(connection.provider === "aws"
                ? {
                    account_id: connection.configuration.account_id,
                    role_arn: connection.configuration.role_arn,
                  }
                : {}),
            },
          });
          setProvider(connection.provider);
          setAdvanced(false);
          setParams({
            page: "Connections",
            provider: connection.provider,
            setup: setup.id,
          });
          return;
        }
        const handoff = await post<Schema["AuthorizationHandoff"]>(
          action,
          input(),
        );
        const target = new URL(handoff.url);
        if (target.origin !== window.location.origin)
          throw new Error(
            "Authorization origin differs from this application. Check the configured public URL.",
          );
        window.location.assign(target.href);
        return;
      }
      const result = await post<Schema["Operation"]>(action, input());
      setNotice(
        result.state
          ? `${human(action)}: ${human(result.state)}${result.error_code ? ` — ${human(result.error_code)}` : ""}`
          : `${human(action)} saved.`,
      );
    });
  }
  const loadError = catalog.error || list.error;
  if (loadError)
    return (
      <ErrorPanel
        error={loadError}
        retry={() => {
          void catalog.refetch();
          void list.refetch();
        }}
      />
    );
  return (
    <section className="connections-workspace">
      <header className="page-heading">
        <div>
          <p className="eyebrow">Evidence access</p>
          <h1>Connections</h1>
          <p className="muted">
            Connect once, choose resources, collect evidence. Access health and
            assessment progress are tracked separately.
          </p>
        </div>
        <Link className="button" to="?page=Sources">
          All evidence sources
        </Link>
      </header>
      {error && (
        <ErrorPanel
          error={error}
          retry={() => {
            void list.refetch();
            void scope.refetch();
          }}
        />
      )}
      <output aria-live="polite">
        {pending ? "Working…" : notice}
        {resumed.data &&
          ` Authorization ${human(resumed.data.state)}${resumed.data.error_code ? `: ${human(resumed.data.error_code)}` : ""}`}
      </output>
      <div className="connection-layout">
        <aside className="panel">
          <h2>Your connections</h2>
          {list.isPending && <p>Loading connections…</p>}
          {(list.data || []).map((item) => (
            <button
              type="button"
              className="connection-card"
              aria-pressed={item.connection.id === selected?.connection.id}
              key={item.connection.id}
              onClick={() => {
                setParams({
                  page: "Connections",
                  connection: item.connection.id,
                });
                setProvider("");
              }}
            >
              <strong>{item.connection.name}</strong>
              <span>
                {human(item.connection.provider)} ·{" "}
                {item.connection.verified_account_id ||
                  item.connection.configuration.account_id}
              </span>
              <span>
                {human(item.connection.access_mode)} ·{" "}
                {human(item.connection.lifecycle)}
              </span>
              <span>Access: {human(item.access.state)}</span>
            </button>
          ))}
          {!list.isPending && !list.data?.length && (
            <p>No connections configured yet.</p>
          )}
          <h3>Add a connection</h3>
          {catalog.data?.map((item) => (
            <button
              type="button"
              className="connection-card"
              disabled={!canConnect}
              key={item.id}
              onClick={() => {
                setProvider(item.id);
                setAdvanced(false);
                setParams({ page: "Connections" });
              }}
            >
              <strong>{item.name}</strong>
              <span>
                {item.server_adapter_available
                  ? "Server integration available"
                  : "Agent-managed access available"}
              </span>
            </button>
          ))}
        </aside>
        <div className="connection-main">
          {guide && (selected || advanced || !standard) && (
            <section className="panel">
              <h2>{guide.name} setup</h2>
              <p>{guide.guide.execution}</p>
              <ol>
                {guide.guide.steps?.map((step) => (
                  <li key={step}>{step}</li>
                ))}
              </ol>
              <details>
                <summary>Required permissions and deployment settings</summary>
                <ul>
                  {guide.permissions?.map((permission) => (
                    <li key={permission}>
                      <code>{permission}</code>
                    </li>
                  ))}
                </ul>
                <p>
                  Callback:{" "}
                  <code>
                    {guide.guide.callback_path || "No OAuth callback required"}
                  </code>
                </p>
                <ul>
                  {guide.guide.configuration_names?.map((name) => (
                    <li key={name}>
                      <code>{name}</code>
                    </li>
                  ))}
                </ul>
              </details>
            </section>
          )}
          {standard && !selected && (
            <>
              <button type="button" onClick={() => setAdvanced(!advanced)}>
                {advanced
                  ? "Use guided setup"
                  : "Advanced / agent-managed setup"}
              </button>
              {!advanced && (
                <ProviderSetup
                  key={provider}
                  service={service}
                  provider={provider}
                  csrf={csrf}
                  canConnect={canConnect}
                />
              )}
            </>
          )}
          {provider && guide && !selected && (!standard || advanced) && (
            <form
              className="panel connection-form"
              onSubmit={(event) => {
                event.preventDefault();
                const form = new FormData(event.currentTarget);
                void run(async () => {
                  const configuration: Record<string, string> = {
                    account_id: String(form.get("account_id") || ""),
                  };
                  for (const field of [
                    "installation_id",
                    "profile",
                    "role_arn",
                    "context",
                    "cluster_id",
                    "server",
                  ]) {
                    const value = String(form.get(field) || "");
                    if (value) configuration[field] = value;
                  }
                  if (provider === "aws") configuration.region = "us-east-1";
                  const created = await post<Detail>("configure", {
                    name: form.get("name"),
                    provider,
                    access_mode: form.get("mode"),
                    configuration,
                    expected_revision: 0,
                    idempotency_key: newID(),
                  });
                  setParams({
                    page: "Connections",
                    connection: created.connection.id,
                  });
                  setProvider("");
                  setNotice(
                    "Connection saved. Verify access before selecting resources.",
                  );
                });
              }}
            >
              <h2>Configure {guide.name}</h2>
              <label>
                Name
                <input name="name" required maxLength={256} />
              </label>
              <label>
                Access mode
                <select name="mode" aria-label="Access mode">
                  {guide.access_modes?.map((mode) => (
                    <option key={mode} value={mode}>
                      {human(mode)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                {provider === "kubernetes"
                  ? "Cluster identity (kube-system namespace UID)"
                  : "Account / customer ID"}
                <input name="account_id" required />
              </label>
              {provider === "github" && (
                <label>
                  GitHub App installation ID
                  <input name="installation_id" inputMode="numeric" />
                </label>
              )}
              {provider === "aws" && (
                <>
                  <p>
                    All AWS inspection is read-only and restricted to us-east-1.
                  </p>
                  <label>
                    Server profile
                    <input name="profile" />
                  </label>
                  <label>
                    Assume-role ARN
                    <input name="role_arn" />
                  </label>
                </>
              )}
              {provider === "kubernetes" && (
                <>
                  <label>
                    Cluster ID (same UID as above)
                    <input name="cluster_id" required />
                  </label>
                  <label>
                    HTTPS API server
                    <input name="server" type="url" required />
                  </label>
                  <label>
                    Trusted server context
                    <input name="context" />
                  </label>
                  <p>
                    For uploaded access, choose Server Managed and use the
                    secure browser handoff after saving.
                  </p>
                </>
              )}
              <button disabled={pending || !canConnect} type="submit">
                Save connection
              </button>
            </form>
          )}
          {selected && (
            <>
              <section className="panel">
                <h2>{selected.connection.name}</h2>
                <p>
                  {selected.discovered_count} discovered ·{" "}
                  {selected.selected_count} selected sources
                </p>
                <p>
                  {human(selected.connection.provider)} ·{" "}
                  {selected.connection.verified_account_id ||
                    selected.connection.configuration.account_id}
                </p>
                <dl>
                  <dt>Access</dt>
                  <dd>{human(selected.access.state)}</dd>
                  <dt>Last checked</dt>
                  <dd>
                    {selected.access.checked_at
                      ? date(selected.access.checked_at)
                      : "Not tested"}
                  </dd>
                  {selected.access.principal && (
                    <>
                      <dt>Server principal</dt>
                      <dd>{selected.access.principal}</dd>
                    </>
                  )}
                  <dt>Configuration revision / credential generation</dt>
                  <dd>
                    {selected.connection.revision} /{" "}
                    {selected.connection.generation}
                  </dd>
                </dl>
                {selected.access.error_code && (
                  <output>{human(selected.access.error_code)}</output>
                )}
                {selected.access.limitations?.map((limitation) => (
                  <p key={limitation}>{limitation}</p>
                ))}
                <div className="connection-actions">
                  {selected.connection.lifecycle === "active" ? (
                    ["authorize", "test", "renew", "disconnect", "delete"].map(
                      (action) => (
                        <button
                          key={action}
                          type="button"
                          disabled={
                            pending ||
                            !canConnect ||
                            (action === "authorize" &&
                              (selected.connection.access_mode !==
                                "server_managed" ||
                                !guide?.server_adapter_available))
                          }
                          onClick={() => void operation(action)}
                        >
                          {action === "authorize" &&
                          selected.connection.provider === "kubernetes"
                            ? "Upload access"
                            : human(action)}
                        </button>
                      ),
                    )
                  ) : (
                    <button
                      type="button"
                      disabled={pending || !canConnect}
                      onClick={() => void operation("restore")}
                    >
                      Restore connection
                    </button>
                  )}
                </div>
                <p className="muted">
                  Renewal updates access only. Disconnect and delete preserve
                  evidence history; remote revocation is separate.
                </p>
                {selected.connection.lifecycle === "active" && (
                  <details>
                    <summary>Edit connection configuration</summary>
                    <form
                      key={selected.connection.id}
                      className="connection-form"
                      onSubmit={(event) => {
                        event.preventDefault();
                        const form = new FormData(event.currentTarget);
                        void run(async () => {
                          const configuration = {
                            ...selected.connection.configuration,
                          };
                          for (const name of [
                            "profile",
                            "role_arn",
                            "context",
                            "server",
                            "installation_id",
                          ] as const) {
                            if (form.has(name))
                              configuration[name] = String(
                                form.get(name) || "",
                              );
                          }
                          await post("configure", {
                            id: selected.connection.id,
                            expected_revision: selected.connection.revision,
                            idempotency_key: newID(),
                            provider: selected.connection.provider,
                            name: form.get("name"),
                            access_mode: form.get("mode"),
                            configuration,
                          });
                          setNotice(
                            "Configuration saved. Verify access again before collection.",
                          );
                        });
                      }}
                    >
                      <p>
                        Account identity is fixed. Reconfiguration clears cached
                        access and requires a new access test or authorization.
                      </p>
                      <label>
                        Name
                        <input
                          name="name"
                          defaultValue={selected.connection.name}
                          required
                        />
                      </label>
                      <label>
                        Access mode
                        <select
                          name="mode"
                          defaultValue={selected.connection.access_mode}
                        >
                          {guide?.access_modes?.map((mode) => (
                            <option key={mode} value={mode}>
                              {human(mode)}
                            </option>
                          ))}
                        </select>
                      </label>
                      {(selected.connection.provider === "github"
                        ? ["installation_id"]
                        : selected.connection.provider === "aws"
                          ? ["profile", "role_arn"]
                          : selected.connection.provider === "kubernetes"
                            ? ["context", "server"]
                            : []
                      ).map((name) => (
                        <label key={name}>
                          {human(name)}
                          <input
                            name={name}
                            defaultValue={
                              (selected.connection.configuration[
                                name as keyof typeof selected.connection.configuration
                              ] as string) || ""
                            }
                          />
                        </label>
                      ))}
                      <button type="submit" disabled={pending || !canConnect}>
                        Save configuration
                      </button>
                    </form>
                  </details>
                )}
              </section>
              <SourcePicker
                key={selected.connection.id}
                service={service}
                detail={selected}
                scopeRevision={scope.data?.revision ?? 0}
                post={post}
                run={run}
                pending={pending}
                canSelect={canConnect && capabilities.includes("edit")}
                canDiscover={canConnect}
                canCheck={capabilities.includes("check")}
              />
            </>
          )}
        </div>
      </div>
    </section>
  );
}

function SourcePicker({
  service,
  detail,
  scopeRevision,
  post,
  run,
  pending,
  canSelect,
  canDiscover,
  canCheck,
}: {
  service: string;
  detail: Detail;
  scopeRevision: number;
  post: <T>(action: string, body: object) => Promise<T>;
  run: (action: () => Promise<void>) => Promise<void>;
  pending: boolean;
  canSelect: boolean;
  canDiscover: boolean;
  canCheck: boolean;
}) {
  const [search, setSearch] = useState("");
  const [cursor, setCursor] = useState("");
  const [selected, setSelected] = useState<Record<string, Resource>>({});
  const [addScope, setAddScope] = useState(false);
  const [pins, setPins] = useState<
    Record<string, { commit: string; paths: string }>
  >({});
  const [session, setSession] = useState<{ id: string; cursor: string } | null>(
    null,
  );
  const [status, setStatus] = useState("");
  const c = detail.connection;
  const inventory = useQuery({
    queryKey: [service, "inventory", c.id, search, cursor],
    queryFn: () =>
      request<Schema["InventoryPage"]>(
        `${api(service)}/connections/${encodeURIComponent(c.id)}/inventory?${new URLSearchParams({ q: search, cursor })}`,
      ),
  });
  async function discover() {
    await run(async () => {
      let current = session;
      if (!current) {
        const created = await post<Schema["DiscoverySession"]>("discover", {
          connection_id: c.id,
          expected_revision: c.revision,
          generation: c.generation,
          idempotency_key: newID(),
        });
        current = { id: created.id, cursor: "" };
      }
      setSession(current);
      const receipt = await post<Schema["DiscoveryReceipt"]>("discover-page", {
        session_id: current.id,
        cursor: current.cursor,
        limit: 25,
        idempotency_key: newID(),
      });
      setSession(
        receipt.next_cursor
          ? { id: current.id, cursor: receipt.next_cursor }
          : null,
      );
      setStatus(
        `Discovery ${human(receipt.state)} · ${receipt.accepted} resources on this page. ${
          receipt.coverage
            ?.filter((coverage) => coverage.error_code)
            .map((coverage) => `${coverage.kind}: ${coverage.error_code}`)
            .join("; ") || ""
        }`,
      );
      await inventory.refetch();
    });
  }
  return (
    <section className="panel">
      <h2>Choose evidence sources</h2>
      <p>Discovery is inventory, not assessment scope or evidence refresh.</p>
      <button
        type="button"
        disabled={
          pending ||
          !canDiscover ||
          c.lifecycle !== "active" ||
          c.access_mode === "agent_managed"
        }
        onClick={() => void discover()}
      >
        {session ? "Continue discovery" : "Discover resources"}
      </button>
      {c.access_mode === "agent_managed" && (
        <p>
          Use MCP discover_connection_sources and submit_connection_discovery
          with your agent’s own tools.
        </p>
      )}
      <output aria-live="polite">{status}</output>
      <label>
        Search resources
        <input
          value={search}
          onChange={(event) => {
            setSearch(event.target.value);
            setCursor("");
          }}
        />
      </label>
      {inventory.error && (
        <ErrorPanel error={inventory.error} retry={() => inventory.refetch()} />
      )}
      {inventory.isPending && <p>Loading inventory…</p>}
      <ul className="connection-resources">
        {inventory.data?.resources?.map((resource) => (
          <li key={`${resource.resource.kind}/${resource.resource.id}`}>
            <label>
              <input
                type="checkbox"
                disabled={
                  !!resource.source_id || !resource.available || !canSelect
                }
                checked={!!selected[resourceKey(resource)]}
                onChange={(event) =>
                  setSelected((previous) => {
                    const next = { ...previous };
                    if (event.target.checked)
                      next[resourceKey(resource)] = resource;
                    else delete next[resourceKey(resource)];
                    return next;
                  })
                }
              />
              <span>
                <strong>{resource.name}</strong>
                <small>
                  {human(resource.resource.kind)} ·{" "}
                  {resource.source_id
                    ? `Selected · ${human(resource.source_lifecycle || "enabled")}`
                    : resource.available
                      ? "Available"
                      : "Unavailable"}
                </small>
                <small>
                  {human(resource.access_mode)} · {date(resource.last_seen_at)}
                </small>
              </span>
            </label>
            {resource.source_id && resource.bound_connection_id === c.id && (
              <div className="connection-actions">
                <Link
                  to={`?page=Sources&source=${encodeURIComponent(resource.source_id)}`}
                >
                  Source history
                </Link>
                <button
                  type="button"
                  disabled={
                    pending ||
                    !canCheck ||
                    resource.source_lifecycle !== "enabled" ||
                    c.lifecycle !== "active" ||
                    c.access_mode === "agent_managed" ||
                    c.provider === "aws"
                  }
                  onClick={() =>
                    void run(async () => {
                      await post("refresh-source", {
                        source_id: resource.source_id,
                        expected_revision: resource.source_revision,
                        scope_revision: scopeRevision,
                        generation: c.generation,
                        idempotency_key: newID(),
                      });
                      setStatus(
                        "New evidence snapshot saved. Existing citations remain unchanged.",
                      );
                    })
                  }
                >
                  Refresh evidence
                </button>
                <button
                  type="button"
                  disabled={pending || !canSelect}
                  onClick={() =>
                    void run(async () => {
                      const action =
                        resource.source_lifecycle === "retired"
                          ? "restore"
                          : "retire";
                      await post(`${action}-source`, {
                        source_id: resource.source_id,
                        expected_revision: resource.source_revision,
                        expected_binding_revision: resource.binding_revision,
                        idempotency_key: newID(),
                      });
                      setStatus(
                        `Source ${action === "retire" ? "retired" : "restored"}; history retained.`,
                      );
                    })
                  }
                >
                  {resource.source_lifecycle === "retired"
                    ? "Restore source"
                    : "Retire source"}
                </button>
              </div>
            )}
          </li>
        ))}
      </ul>
      {!inventory.isPending && !inventory.data?.resources?.length && (
        <p>No matching resources. Discover inventory or adjust your search.</p>
      )}
      <div className="connection-actions">
        <button type="button" disabled={!cursor} onClick={() => setCursor("")}>
          First page
        </button>
        <button
          type="button"
          disabled={!inventory.data?.next_cursor}
          onClick={() => setCursor(inventory.data?.next_cursor || "")}
        >
          Next page
        </button>
      </div>
      {Object.values(selected).map((resource) => (
        <div className="connection-selection" key={resourceKey(resource)}>
          <strong>{resource.name}</strong>
          <button
            type="button"
            onClick={() =>
              setSelected((previous) => {
                const next = { ...previous };
                delete next[resourceKey(resource)];
                return next;
              })
            }
          >
            Remove selection
          </button>
          {c.provider === "github" && (
            <>
              <label>
                Full commit SHA
                <input
                  pattern="[a-f0-9]{40}"
                  value={pins[resourceKey(resource)]?.commit || ""}
                  onChange={(event) =>
                    setPins((previous) => ({
                      ...previous,
                      [resourceKey(resource)]: {
                        paths: previous[resourceKey(resource)]?.paths || "",
                        commit: event.target.value,
                      },
                    }))
                  }
                />
              </label>
              <label>
                Evidence paths (one per line)
                <textarea
                  value={pins[resourceKey(resource)]?.paths || ""}
                  onChange={(event) =>
                    setPins((previous) => ({
                      ...previous,
                      [resourceKey(resource)]: {
                        commit: previous[resourceKey(resource)]?.commit || "",
                        paths: event.target.value,
                      },
                    }))
                  }
                />
              </label>
            </>
          )}
        </div>
      ))}
      <label>
        <input
          type="checkbox"
          checked={addScope}
          onChange={(event) => setAddScope(event.target.checked)}
        />
        Explicitly add selected resources to assessment scope
      </label>
      <p>
        {Object.keys(selected).length} selected across pages · scope revision{" "}
        {scopeRevision}
      </p>
      <button
        type="button"
        disabled={pending || !canSelect || !Object.keys(selected).length}
        onClick={() =>
          void run(async () => {
            const sources = Object.values(selected).map((resource) => {
              const source: Record<string, unknown> = {
                type: c.provider,
                name: resource.name,
                locator: resource.locator,
                enabled: true,
                freshness_hours: 24,
                scope_revision: scopeRevision,
                access_hint: "",
                paths: [],
                requirement_ids: [],
              };
              if (c.provider === "github") {
                const pin = pins[resourceKey(resource)];
                if (
                  !pin ||
                  !/^[a-f0-9]{40}$/.test(pin.commit) ||
                  !pin.paths.trim()
                )
                  throw new Error(
                    `Enter a full commit and evidence paths for ${resource.name}.`,
                  );
                Object.assign(source, {
                  type: "repository",
                  repository: resource.name,
                  commit: pin.commit,
                  paths: pin.paths
                    .split("\n")
                    .map((value) => value.trim())
                    .filter(Boolean),
                });
              }
              if (c.provider === "aws")
                Object.assign(source, {
                  type:
                    resource.resource.kind === "eks_cluster"
                      ? "kubernetes"
                      : "aws",
                  account_id: resource.resource.tenant,
                  region: "us-east-1",
                  cluster:
                    resource.resource.kind === "eks_cluster"
                      ? resource.name
                      : "",
                });
              if (c.provider === "kubernetes")
                Object.assign(source, {
                  cluster_id: resource.resource.tenant,
                  namespace:
                    resource.resource.kind === "namespace" ? resource.name : "",
                  namespace_uid:
                    resource.resource.kind === "namespace"
                      ? resource.resource.id
                      : "",
                });
              return {
                service_id: service,
                source_id: newID(),
                connection_id: c.id,
                generation: c.generation,
                expected_revision: 0,
                expected_binding_revision: 0,
                idempotency_key: "",
                resource: resource.resource,
                source,
              };
            });
            await post("select", {
              expected_scope_revision: scopeRevision,
              idempotency_key: newID(),
              add_to_scope: addScope,
              sources,
            });
            setSelected({});
            setStatus(
              "Sources selected. Evidence collection and review remain separate.",
            );
          })
        }
      >
        Select sources
      </button>
    </section>
  );
}
