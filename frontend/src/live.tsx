import * as Dialog from "@radix-ui/react-dialog";
import { useQuery } from "@tanstack/react-query";
import { ArrowUpRight, Menu, ShieldCheck, X } from "lucide-react";
import { type ReactNode, useState } from "react";
import {
  Link,
  useLocation,
  useNavigate,
  useSearchParams,
} from "react-router-dom";
import type { Value } from "./api";
import {
  api,
  type Bootstrap,
  date,
  human,
  type Overview,
  type Page,
  type RecordItem,
  type RecordPage,
  type Rule,
  request,
  text,
} from "./api";
import type { Status } from "./fixtures";

function asObject(v: Value | undefined): Record<string, Value> {
  return v !== null && typeof v === "object" && !Array.isArray(v) ? v : {};
}

import { ConnectionsPage } from "./connections";
import { EvidenceCoveragePanel } from "./evidence-coverage";
import { ActionForm, ErrorPanel, parseValue, Readable } from "./forms";
import { ImplementationAssessmentPanel } from "./implementation-assessment";
import { OrganizationPage } from "./organization";
import { PlaybookActions } from "./playbook-actions";
import { ReadinessPage } from "./readiness";
import {
  ContextGaps,
  GuidanceState,
  ScopedReadiness,
} from "./readiness-context";
import { RequirementActions } from "./requirement-actions";
import { StatusBadge } from "./workspace";

const pages = [
  "Overview",
  "Requirements",
  "Readiness",
  "Evidence",
  "Activity",
  "Sources",
  "Connections",
  "Settings",
  "Exports",
];
const legacyViews: Record<string, string> = {
  sources: "Sources",
  settings: "Settings",
  collection: "Activity",
  history: "Exports",
};
const priorities: Status[] = [
  "failing",
  "blocked",
  "needs_refresh",
  "needs_evidence",
  "needs_scoping",
  "in_review",
  "in_progress",
  "not_started",
  "passing",
  "not_applicable",
];
function Loading() {
  return (
    <output className="panel loading-state">
      Loading your workspace…
      <div className="skeleton" />
    </output>
  );
}
export function LiveApp() {
  const location = useLocation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [menu, setMenu] = useState(false);
  const parts = location.pathname.split("/");
  const id = parts[1] === "services" ? parts[2] : "";
  const requirement = parts[3] === "requirements" ? parts[4] : "";
  const run = parts[3] === "runs" ? parts[4] : "";
  const page =
    params.get("page") ||
    legacyViews[params.get("view") || ""] ||
    (params.has("kind") || params.has("gaps") ? "Requirements" : "Overview");
  const bootstrap = useQuery({
    queryKey: ["bootstrap"],
    queryFn: () => request<Bootstrap>("/api/ui/v1/bootstrap"),
  });
  if (bootstrap.isPending)
    return (
      <main>
        <Loading />
      </main>
    );
  if (bootstrap.error)
    return (
      <main>
        <ErrorPanel error={bootstrap.error} retry={() => bootstrap.refetch()} />
      </main>
    );
  const data = bootstrap.data;
  const service = data.services.find((s) => s.id === id);
  const base = `/services/${id}`;
  const nav = (
    <>
      <div className="brand">
        <ShieldCheck />
        <span>
          Assessment<span className="muted brand-sub">Evidence workspace</span>
        </span>
      </div>
      <label className="service-card">
        Service
        <select
          value={id}
          onChange={(e) => {
            navigate(e.target.value ? `/services/${e.target.value}` : "/");
            setMenu(false);
          }}
        >
          <option value="">Choose a service</option>
          {data.services.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      <nav aria-label="Main navigation">
        {id &&
          pages
            .filter((p) => p !== "Readiness" || data.guidance_enabled)
            .map((p) => (
              <Link
                key={p}
                onClick={() => setMenu(false)}
                to={`${base}?page=${p}`}
                className={`nav-link ${
                  p === page && !requirement && !run ? "selected" : ""
                }`}
              >
                {p}
              </Link>
            ))}
        <Link className="nav-link" to="/">
          All services
        </Link>
        {data.can_manage_organization && (
          <Link
            className="nav-link"
            to="/organization"
            onClick={() => setMenu(false)}
          >
            Organization & users
          </Link>
        )}
      </nav>
      <div className="sidebar-footer">Evidence · Findings · Human review</div>
    </>
  );
  return (
    <div className="app">
      <a className="skip" href="#main">
        Skip to content
      </a>
      <aside className="sidebar">{nav}</aside>
      <div className="workspace">
        <header className="topbar">
          <Dialog.Root open={menu} onOpenChange={setMenu}>
            <Dialog.Trigger
              className="icon-button mobile-menu"
              aria-label="Open navigation"
            >
              <Menu />
            </Dialog.Trigger>
            <Dialog.Portal>
              <Dialog.Overlay className="overlay" />
              <Dialog.Content className="drawer">
                <Dialog.Title className="sr-only">
                  Workspace navigation
                </Dialog.Title>
                <Dialog.Description className="sr-only">
                  Select a service and page
                </Dialog.Description>
                <Dialog.Close
                  className="icon-button close"
                  aria-label="Close navigation"
                >
                  <X />
                </Dialog.Close>
                {nav}
              </Dialog.Content>
            </Dialog.Portal>
          </Dialog.Root>
          <Link to={id ? base : "/"}>{service?.name || "Services"}</Link>
          {data.profiles?.[id] && (
            <Link className="small muted" to={`${base}?page=Settings`}>
              {data.profiles[id]}
            </Link>
          )}
          <span className="muted">
            / {requirement ? "Requirement" : run ? "Collection run" : page}
          </span>
        </header>
        <main id="main" tabIndex={-1}>
          {location.pathname === "/organization" ? (
            data.can_manage_organization ? (
              <OrganizationPage csrf={data.csrf} />
            ) : (
              <section className="panel">
                <h1>Administrator access required</h1>
                <p>Your current role cannot manage organization users.</p>
                <Link to="/">Return to services</Link>
              </section>
            )
          ) : !id ? (
            <>
              <Heading
                title="Your services"
                description="Choose an assessment or start a new one."
              />
              <div className="service-grid">
                {data.services.map((s) => (
                  <Link className="panel" key={s.id} to={`/services/${s.id}`}>
                    <h2>{s.name}</h2>
                    <p className="text-link">
                      Open assessment <ArrowUpRight size={16} />
                    </p>
                  </Link>
                ))}
              </div>
              {data.can_create && (
                <ActionForm
                  csrf={data.csrf}
                  form={{
                    Action: "/services",
                    Title: "Start an assessment",
                    Button: "Create service",
                    CSRF: data.csrf,
                    Fields: [
                      {
                        Name: "name",
                        Label: "Service name",
                        Value: "",
                        Type: "",
                        Options: null,
                      },
                    ],
                  }}
                />
              )}
            </>
          ) : requirement ? (
            <LegacyPage
              service={id}
              endpoint={`requirement/${requirement}`}
              bootstrap={data}
            />
          ) : run ? (
            <LegacyPage service={id} endpoint={`run/${run}`} bootstrap={data} />
          ) : page === "Overview" || page === "Requirements" ? (
            <ProgressPage service={id} listing={page === "Requirements"} />
          ) : page === "Readiness" && data.guidance_enabled ? (
            <ReadinessPage
              service={id}
              csrf={data.csrf}
              canEdit={(data.capabilities[id] || []).includes("edit")}
            />
          ) : page === "Connections" ? (
            <ConnectionsPage
              service={id}
              csrf={data.csrf}
              capabilities={data.capabilities[id] || []}
            />
          ) : page === "Evidence" ? (
            <RecordList service={id} kind="evidence" />
          ) : page === "Activity" ? (
            <>
              <RecordList service={id} kind="rule_result" />
              <LegacyPage service={id} endpoint="collection" bootstrap={data} />
            </>
          ) : (
            <LegacyPage
              service={id}
              endpoint={page === "Exports" ? "history" : page.toLowerCase()}
              bootstrap={data}
            />
          )}
        </main>
      </div>
    </div>
  );
}
function Heading({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children?: ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        <p className="eyebrow">ASSESSMENT WORKSPACE</p>
        <h1>{title}</h1>
        <p className="muted">{description}</p>
      </div>
      {children}
    </div>
  );
}
function ProgressPage({
  service,
  listing,
}: {
  service: string;
  listing: boolean;
}) {
  const [params, setParams] = useSearchParams();
  const data = useQuery({
    queryKey: ["overview", service],
    queryFn: () => request<Overview>(`${api(service)}/overview`),
  });
  if (data.isPending) return <Loading />;
  if (data.error && !data.data)
    return <ErrorPanel error={data.error} retry={() => data.refetch()} />;
  if (!data.data) return null;
  const model = data.data;
  const c = model.counts;
  const base = `/services/${service}`;
  function filter(key: string, value: string) {
    const next = new URLSearchParams(params);
    if (key !== "offset") next.delete("offset");
    next.set("page", "Requirements");
    if (value) next.set(key, value);
    else next.delete(key);
    setParams(next);
  }
  const matches = model.rules.filter(
    (r) =>
      (!params.get("status") || r.state.status === params.get("status")) &&
      (params.get("gaps") !== "1" || !r.state.human_reviewed || r.open_gap) &&
      (!params.get("evidence_complete") || r.state.evidence_complete) &&
      (!params.get("review_remaining") || r.state.review_remaining) &&
      (!params.get("category") || r.category === params.get("category")) &&
      (!params.get("applicability") ||
        r.applicability === params.get("applicability")) &&
      (!params.get("kind") || r.kind === params.get("kind")) &&
      (!params.get("q") ||
        `${r.name} ${r.id}`
          .toLowerCase()
          .includes((params.get("q") || "").toLowerCase())),
  );
  const offset = Math.max(0, Number(params.get("offset")) || 0);
  const ordered = [...model.rules]
    .filter((r) => r.applicability === "required")
    .sort(
      (a, b) =>
        priorities.indexOf(a.state.status) - priorities.indexOf(b.state.status),
    );
  const link = (query: string) =>
    `${base}?page=Requirements&applicability=required&${query}`;
  return (
    <>
      <Heading
        title={listing ? "Requirements" : "See where you stand."}
        description={
          listing
            ? "Find what is complete, what is missing, and the next useful action."
            : "Your progress, evidence, and priorities in one place."
        }
      >
        <Link className="button" to={`${base}?page=Sources`}>
          Connect evidence sources <ArrowUpRight size={16} />
        </Link>
      </Heading>
      {data.error && (
        <ErrorPanel error={data.error} retry={() => data.refetch()} />
      )}
      <p className="small muted update-time">Updated {date(model.as_of)}</p>
      {model.setup_required ? (
        <section className="panel empty">
          <h2>Choose a profile to begin</h2>
          <p>
            Define the assessment profile and scope, then connect evidence
            sources.
          </p>
          <Link className="text-link" to={`${base}?page=Settings`}>
            Set up this service
          </Link>
        </section>
      ) : listing ? (
        <>
          <div className="panel filters">
            <label>
              Search
              <input
                value={params.get("q") || ""}
                onChange={(e) => filter("q", e.target.value)}
                placeholder="Find a requirement"
              />
            </label>
            <label>
              Status
              <select
                value={params.get("status") || ""}
                onChange={(e) => filter("status", e.target.value)}
              >
                <option value="">All statuses</option>
                {priorities.map((s) => (
                  <option key={s} value={s}>
                    {human(s)}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Applicability
              <select
                value={params.get("applicability") || ""}
                onChange={(e) => filter("applicability", e.target.value)}
              >
                <option value="">All requirements</option>
                {[
                  "required",
                  "recommended",
                  "optional",
                  "unresolved",
                  "not_applicable",
                ].map((s) => (
                  <option key={s} value={s}>
                    {human(s)}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Category
              <select
                value={params.get("category") || ""}
                onChange={(e) => filter("category", e.target.value)}
              >
                <option value="">All categories</option>
                {Object.keys(model.categories)
                  .sort()
                  .map((s) => (
                    <option key={s}>{s}</option>
                  ))}
              </select>
            </label>
          </div>
          <section className="panel">
            <h2>{matches.length} requirements</h2>
            {matches.slice(offset, offset + 25).map((r) => (
              <RuleRow key={r.id} rule={r} service={service} />
            ))}
            {!matches.length && (
              <div className="empty">
                <h2>No matching requirements</h2>
                <Link className="text-link" to={`${base}?page=Requirements`}>
                  Clear filters
                </Link>
              </div>
            )}
            <Pager
              offset={offset}
              more={offset + 25 < matches.length}
              change={(v) => filter("offset", String(v))}
            />
          </section>
        </>
      ) : (
        <>
          <div className="metrics">
            {[
              ["Passing requirements", c.passing, `status=passing`],
              ["Evidence complete", c.evidence_complete, "evidence_complete=1"],
              ["Reviews remaining", c.reviews_remaining, "review_remaining=1"],
            ].map(([label, value, query]) => (
              <Link
                className="panel metric"
                key={label}
                to={link(String(query))}
              >
                <span className="muted">{label}</span>
                <div className="metric-value">
                  {value}
                  <span> / {c.required}</span>
                </div>
                <span className="metric-caption">
                  Applicable required requirements <ArrowUpRight size={15} />
                </span>
              </Link>
            ))}
          </div>
          <div className="panel progress-panel">
            <Link className="text-link" to={`${base}?page=Evidence`}>
              {model.evidence_records} evidence records gathered{" "}
              <ArrowUpRight size={15} />
            </Link>
            <div className="section-heading">
              <h2>Assessment progress</h2>
              <strong>
                {c.required
                  ? `${Math.round((c.passing / c.required) * 100)}% passing`
                  : "No required requirements"}
              </strong>
            </div>
            <div
              className="progress-track"
              role="img"
              aria-label={`${c.passing} of ${c.required} required requirements passing`}
            >
              <span
                style={{
                  width: `${c.required ? (c.passing / c.required) * 100 : 0}%`,
                }}
              />
              <span
                style={{
                  width: `${
                    c.required
                      ? ((c.statuses?.in_review || 0) / c.required) * 100
                      : 0
                  }%`,
                }}
              />
              <span style={{ flex: 1 }} />
            </div>
            <div className="legend">
              {Object.entries(c.statuses || {}).map(([s, count]) => (
                <Link key={s} to={link(`status=${s}`)}>
                  {count} {human(s)}
                </Link>
              ))}
            </div>
          </div>
          {c.scope_decisions > 0 && (
            <Link
              className="panel attention"
              to={`${base}?page=Requirements&status=needs_scoping`}
            >
              <strong>{c.scope_decisions} scope decisions needed</strong>
              <span className="muted">
                Resolve applicability to complete the assessment.
              </span>
              <ArrowUpRight />
            </Link>
          )}
          <div className="legend">
            <span className="muted">Additional work:</span>
            <Link
              className="text-link"
              to={`${base}?page=Requirements&applicability=recommended`}
            >
              {c.recommended} recommended requirements
            </Link>
            <Link
              className="text-link"
              to={`${base}?page=Requirements&applicability=optional`}
            >
              {c.optional} optional requirements
            </Link>
          </div>
          <div className="columns">
            <section className="panel">
              <h2>What needs attention</h2>
              {ordered
                .filter((r) => r.state.status !== "passing")
                .slice(0, 5)
                .map((r) => (
                  <RuleRow key={r.id} rule={r} service={service} />
                ))}
            </section>
            <section className="panel">
              <h2>Requirement categories</h2>
              {Object.entries(model.categories)
                .filter(([, count]) => count.required > 0)
                .sort(([a], [b]) => a.localeCompare(b))
                .map(([category, count]) => (
                  <Link
                    className="attention"
                    key={category}
                    to={link(`category=${category}`)}
                  >
                    <strong>{category}</strong>
                    <span className="muted">
                      {count.passing} / {count.required} required passing
                    </span>
                    <ArrowUpRight size={16} />
                  </Link>
                ))}
            </section>
          </div>
          <RecordList service={service} kind="rule_result" compact />
        </>
      )}
    </>
  );
}
function RuleRow({ rule, service }: { rule: Rule; service: string }) {
  return (
    <Link
      className="requirement-row"
      to={`/services/${service}/requirements/${rule.id}`}
    >
      <div>
        <span className="small muted">
          {rule.id} · {human(rule.applicability)}
        </span>
        <h2>{rule.name}</h2>
        <p className="muted">{rule.state.reason}</p>
        <GuidanceState
          state={rule.state.guidance_state}
          reason={rule.state.guidance_reason}
        />
        <ScopedReadiness progress={rule.state.readiness} compact />
        <ContextGaps gaps={rule.state.context_gaps} />
        {rule.state.next_steps?.[0] && (
          <p className="small muted">{rule.state.next_steps[0].explanation}</p>
        )}
        <span className="text-link">
          {rule.state.action} <ArrowUpRight size={14} />
        </span>
      </div>
      <div>
        <StatusBadge status={rule.state.status} />
        {rule.state.secondary?.map((label) => (
          <p className="small muted" key={label}>
            {label}
          </p>
        ))}
      </div>
    </Link>
  );
}
function Pager({
  offset,
  more,
  change,
}: {
  offset: number;
  more: boolean;
  change: (offset: number) => void;
}) {
  return (
    <div className="pagination">
      <button
        type="button"
        disabled={offset === 0}
        onClick={() => change(Math.max(0, offset - 25))}
      >
        Previous
      </button>
      <span aria-live="polite">Page {Math.floor(offset / 25) + 1}</span>
      <button
        type="button"
        disabled={!more}
        onClick={() => change(offset + 25)}
      >
        Next
      </button>
    </div>
  );
}
function RecordList({
  service,
  kind,
  compact = false,
}: {
  service: string;
  kind: string;
  compact?: boolean;
}) {
  const [params, setParams] = useSearchParams();
  const offset = Number(params.get("offset")) || 0;
  const search = params.get("q") || "";
  function setFilter(key: string, value: string) {
    const p = new URLSearchParams(params);
    p.set(key, value);
    if (key !== "offset") p.delete("offset");
    setParams(p);
  }
  const setOffset = (value: number) => setFilter("offset", String(value));
  const [selected, setSelected] = useState<RecordItem>();
  const data = useQuery({
    queryKey: ["records", service, kind, offset, params.toString()],
    queryFn: () =>
      request<RecordPage>(
        `${api(service)}/records/${kind}?${params.toString()}`,
      ),
    placeholderData: (previous, query) =>
      query?.queryKey[1] === service && query?.queryKey[2] === kind
        ? previous
        : undefined,
  });
  if (data.isPending) return <Loading />;
  if (data.error && !data.data)
    return <ErrorPanel error={data.error} retry={() => data.refetch()} />;
  if (!data.data) return null;
  const items = data.data.items || [];
  const hasFilters = !!(
    search ||
    params.get("freshness") ||
    params.get("type") ||
    params.get("source") ||
    params.get("requirement")
  );
  return (
    <>
      <Heading
        title={kind === "evidence" ? "Evidence" : "Latest agent results"}
        description={
          kind === "evidence"
            ? "Inspect gathered records, provenance, and supporting content."
            : "Saved investigations and their evidence outcomes."
        }
      />
      <section className="panel">
        <button
          type="button"
          onClick={() => data.refetch()}
          disabled={data.isFetching}
        >
          {data.isFetching ? "Refreshing…" : "Refresh records"}
        </button>
        {data.error && (
          <>
            <p className="muted">
              Showing results from{" "}
              {date(new Date(data.dataUpdatedAt).toISOString())}.
            </p>
            <ErrorPanel error={data.error} retry={() => data.refetch()} />
          </>
        )}
        {data.isPlaceholderData && <output>Updating filters…</output>}
        {!compact && (
          <label className="input-label">
            {kind === "evidence" ? "Search evidence" : "Search investigations"}
            <input
              value={search}
              onChange={(e) => setFilter("q", e.target.value)}
            />
          </label>
        )}
        {kind === "evidence" && (
          <div className="filters">
            <label>
              Freshness
              <select
                value={params.get("freshness") || ""}
                onChange={(e) => setFilter("freshness", e.target.value)}
              >
                <option value="">Any freshness</option>
                <option value="current">Current</option>
                <option value="needs_refresh">Needs refresh</option>
                <option value="unknown">Freshness not established</option>
              </select>
            </label>
            <label>
              Evidence type
              <select
                value={params.get("type") || ""}
                onChange={(e) => setFilter("type", e.target.value)}
              >
                <option value="">All types</option>
                <option value="agent_observation">Agent observations</option>
                <option value="collector_observation">Server collection</option>
                <option value="unverified_reference">
                  External references
                </option>
              </select>
            </label>
            <label>
              Source
              <input
                value={params.get("source") || ""}
                onChange={(e) => setFilter("source", e.target.value)}
                placeholder="Exact source name"
              />
            </label>
            <label>
              Requirement
              <input
                value={params.get("requirement") || ""}
                onChange={(e) => setFilter("requirement", e.target.value)}
                placeholder="Requirement identifier"
              />
            </label>
          </div>
        )}
        {(compact ? items.slice(0, 4) : items).map((item) => (
          <article
            className="requirement-row"
            key={`${item.id}:${item.revision}`}
          >
            <div>
              <span className="small muted">
                {date(item.created_at)} · Revision {item.revision}
              </span>
              <h2>
                {kind === "rule_result"
                  ? `${item.requirement_id} · ${human(
                      text(asObject(asObject(item.data.receipt).work).state),
                    )} investigation`
                  : text(item.data.description)
                      .replace(/^(aws|github|kubernetes)\./, "$1 · ")
                      .replaceAll(
                        ".transit-encryption",
                        " · encryption in transit",
                      ) ||
                    item.requirement_id ||
                    "Evidence record"}
              </h2>
              {kind === "rule_result" && (
                <>
                  <p className="muted">
                    {
                      text(asObject(item.data.submission).explanation).split(
                        /(?<=\.)\s/,
                      )[0]
                    }
                  </p>
                  <details>
                    <summary>Read full investigation</summary>
                    <Readable
                      value={asObject(item.data.submission).explanation || ""}
                    />
                  </details>
                </>
              )}
              <p className="muted">
                {text(item.data.source) ||
                  human(text(asObject(asObject(item.data.receipt).work).state))}
              </p>
              {kind === "rule_result" && (
                <p className="small muted">
                  Cited{" "}
                  {Array.isArray(asObject(item.data.receipt).evidence)
                    ? (asObject(item.data.receipt).evidence as Value[]).length
                    : 0}{" "}
                  evidence records
                </p>
              )}
              {item.freshness && (
                <span className="badge">
                  {item.freshness === "unknown"
                    ? "Freshness not established"
                    : human(item.freshness)}
                </span>
              )}
              {item.linked_requirements && (
                <p className="small muted">
                  Linked to {item.linked_requirements.length} requirements:{" "}
                  {item.linked_requirements.map((id) => (
                    <Link
                      key={id}
                      className="text-link"
                      to={`/services/${service}/requirements/${id}`}
                    >
                      {id}{" "}
                    </Link>
                  ))}
                </p>
              )}
              {item.requirement_id && (
                <Link
                  className="text-link"
                  to={`/services/${service}/requirements/${item.requirement_id}`}
                >
                  {item.requirement_id}
                </Link>
              )}
            </div>
            {kind === "evidence" ? (
              <button type="button" onClick={() => setSelected(item)}>
                Inspect evidence
              </button>
            ) : (
              <span className="badge">
                {human(
                  text(asObject(asObject(item.data.receipt).work).assessment) ||
                    "Saved",
                )}
              </span>
            )}
          </article>
        ))}
        {items.length === 0 && (
          <div className="empty">
            <h2>{hasFilters ? "No matching records" : "No records yet"}</h2>
            <p className="muted">
              {hasFilters
                ? "Clear your search or change the filters."
                : "Ask an agent to investigate the configured sources."}
            </p>
            {hasFilters && (
              <Link
                className="text-link"
                to={`/services/${service}?page=${
                  kind === "evidence" ? "Evidence" : "Activity"
                }`}
              >
                Clear filters
              </Link>
            )}
          </div>
        )}
        {!compact && (
          <Pager offset={offset} more={data.data.has_more} change={setOffset} />
        )}
      </section>
      {selected && (
        <EvidencePreview
          service={service}
          record={selected}
          close={() => setSelected(undefined)}
        />
      )}
    </>
  );
}
function PointerPreview({
  value,
  path = "",
  selected,
}: {
  value: Value;
  path?: string;
  selected?: string | null;
}) {
  if (value !== null && typeof value === "object")
    return (
      <dl className={`readable ${selected === path ? "cited-field" : ""}`}>
        {Object.entries(value).map(([key, item]) => {
          const next = `${path}/${key
            .replaceAll("~", "~0")
            .replaceAll("/", "~1")}`;
          return (
            <div className={selected === next ? "cited-field" : ""} key={next}>
              <dt>
                <code>{next}</code>
              </dt>
              <dd>
                <PointerPreview value={item} path={next} selected={selected} />
              </dd>
            </div>
          );
        })}
      </dl>
    );
  return (
    <span className={selected === path ? "cited-field" : ""}>
      {value === null ? "null" : String(value)}
    </span>
  );
}
function EvidencePreview({
  service,
  record,
  close,
  citation,
}: {
  service: string;
  record: RecordItem;
  close: () => void;
  citation?: { StartLine: number; EndLine: number; JSONPointer: string | null };
}) {
  const body = useQuery({
    queryKey: ["artifact", service, record.id, record.revision],
    queryFn: () =>
      request<{ content: string }>(
        `${api(service)}/preview/${record.id}?revision=${record.revision}`,
      ),
    enabled: !!record.data.artifact_key,
  });
  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="overlay" />
        <Dialog.Content className="evidence-dialog">
          <Dialog.Close
            className="icon-button close"
            aria-label="Close evidence"
          >
            <X />
          </Dialog.Close>
          <Dialog.Title>
            {text(record.data.description) || "Evidence preview"}
          </Dialog.Title>
          <Dialog.Description>
            Revision {record.revision} · Observed{" "}
            {date(text(record.data.observed_at) || record.created_at)}
          </Dialog.Description>
          <p className="muted">{text(record.data.source)}</p>
          {citation && (
            <p className="badge">
              {citation.JSONPointer !== null
                ? `JSON path: ${citation.JSONPointer || "/"}`
                : citation.StartLine
                  ? `Lines ${citation.StartLine}–${citation.EndLine}`
                  : "Whole artifact"}
            </p>
          )}
          {body.isFetching && <Loading />}
          {body.error && (
            <ErrorPanel error={body.error} retry={() => body.refetch()} />
          )}{" "}
          {body.data && (
            <div className="artifact-reader">
              {body.data.content.trimStart().startsWith("{") ||
              body.data.content.trimStart().startsWith("[") ? (
                <PointerPreview
                  value={parseValue(body.data.content)}
                  selected={citation?.JSONPointer}
                />
              ) : (
                <ol>
                  {body.data.content.split("\n").map((line, i) => (
                    <li
                      // biome-ignore lint/suspicious/noArrayIndexKey: immutable artifact line numbers are stable citation identities
                      key={`line-${i}`}
                      id={`line-${i + 1}`}
                      className={
                        citation &&
                        i + 1 >= citation.StartLine &&
                        i + 1 <= citation.EndLine
                          ? "cited-line"
                          : ""
                      }
                    >
                      <code>{line || " "}</code>
                    </li>
                  ))}
                </ol>
              )}
            </div>
          )}
          <details>
            <summary>Provenance and exact references</summary>
            <Readable value={record.data} />
          </details>
          {record.data.artifact_key ? (
            <a
              className="text-link"
              href={`/services/${service}/artifacts/${record.id}?revision=${record.revision}`}
            >
              Download original evidence
            </a>
          ) : text(record.data.url).startsWith("https://") ? (
            <a className="text-link" href={text(record.data.url)}>
              Open source document
            </a>
          ) : (
            <p className="muted">
              No file is stored for this reference. Ask an agent to gather the
              source content and submit a cited artifact.
            </p>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
function CitationList({
  service,
  citations,
}: {
  service: string;
  citations: NonNullable<Page["Citations"]>;
}) {
  const [selected, setSelected] = useState<(typeof citations)[number]>();
  return (
    <section className="panel">
      <h2>Exact evidence citations</h2>
      {citations.map((c) => (
        <button
          type="button"
          key={`${c.Evidence.id}:${c.Evidence.revision}:${c.StartLine}:${c.JSONPointer}:${c.Label}`}
          onClick={() => setSelected(c)}
        >
          {c.Label} · {text(c.Evidence.data.description) || "Evidence"} ·
          revision {c.Evidence.revision} ·{" "}
          {c.JSONPointer !== null
            ? `path ${c.JSONPointer}`
            : c.StartLine
              ? `lines ${c.StartLine}–${c.EndLine}`
              : "whole artifact"}
        </button>
      ))}
      {selected && (
        <EvidencePreview
          service={service}
          record={selected.Evidence}
          citation={selected}
          close={() => setSelected(undefined)}
        />
      )}
    </section>
  );
}
function LegacyPage({
  service,
  endpoint,
  bootstrap,
}: {
  service: string;
  endpoint: string;
  bootstrap: Bootstrap;
}) {
  const [params, setParams] = useSearchParams();
  const location = useLocation();
  const data = useQuery({
    queryKey: ["page", service, endpoint, location.search],
    queryFn: () =>
      request<Page>(`${api(service)}/pages/${endpoint}${location.search}`),
    placeholderData: (previous, query) =>
      query?.queryKey[1] === service && query?.queryKey[2] === endpoint
        ? previous
        : undefined,
    refetchInterval: (query) =>
      query.state.data?.Metrics?.some(
        (m) =>
          m.Label === "Run state" &&
          ["queued", "running", "retry"].includes(m.Value),
      )
        ? 3000
        : false,
  });
  if (data.isPending) return <Loading />;
  if (data.error && !data.data)
    return <ErrorPanel error={data.error} retry={() => data.refetch()} />;
  if (!data.data) return null;
  const p = data.data;
  const caps = bootstrap.capabilities[service] || [];
  const permission = (action: string) =>
    action.endsWith("/review") || action.endsWith("/assessment-decision")
      ? "review"
      : /\/(profile|preview-profile|token|revoke-token|destination|assessor-designation)$/.test(
            action,
          )
        ? "admin"
        : /\/(check|collect)$/.test(action)
          ? "check"
          : action.endsWith("/prepare")
            ? "export"
            : "edit";
  return (
    <>
      <Heading
        title={
          p.Detail
            ? p.Title
            : endpoint === "sources"
              ? "Evidence sources"
              : endpoint === "settings"
                ? "Service settings"
                : endpoint === "history"
                  ? "Exports and history"
                  : endpoint === "collection"
                    ? "Automatic collection"
                    : p.Title
        }
        description={
          p.Detail
            ? "Understand the outcome, inspect evidence, and record your review."
            : endpoint === "sources"
              ? "Help agents reach the right evidence."
              : endpoint === "settings"
                ? "Choose the assessment boundary and manage agent access."
                : endpoint === "history"
                  ? "Prepare and download assessment snapshots at exact revisions."
                  : "Inspect automatic collection results and retry incomplete reads."
        }
      />
      {endpoint.startsWith("requirement/") && <RequirementActions page={p} />}
      {p.Implementation && (
        <ImplementationAssessmentPanel context={p.Implementation} />
      )}
      {p.Guidance && (
        <section className="panel" aria-label="Requirement guidance">
          <h2>{p.Guidance.label || "Requirement guidance"}</h2>
          {p.Guidance.caveat && <p>{p.Guidance.caveat}</p>}
          <GuidanceState state={p.Guidance.state} reason={p.Guidance.reason} />
          <ScopedReadiness progress={p.Guidance.progress} />
          {p.Guidance.guidance?.playbooks?.map((book) => (
            <div key={book.id}>
              <p>{book.purpose.text}</p>
              <p>{book.applicability.text}</p>
              {p.Guidance?.guidance && (
                <PlaybookActions
                  service={service}
                  book={book}
                  guidance={p.Guidance.guidance}
                />
              )}
              {book.review && (
                <>
                  <p>
                    Reviewed by {book.review.reviewer_type} ·{" "}
                    {book.review.reviewed_at}
                  </p>
                  {book.review.comparison.limitations?.map((text) => (
                    <p key={text}>{text}</p>
                  ))}
                  {book.review.comparison.uncertainties?.map((item) => (
                    <p key={item.finding}>Uncertainty: {item.finding}</p>
                  ))}
                </>
              )}
            </div>
          ))}
          {p.Guidance.guidance && (
            <details>
              <summary>Official sources and review details</summary>
              {p.Guidance.guidance.citations?.map((citation) => (
                <div key={citation.id}>
                  <h3>
                    {citation.path} · {citation.anchor}
                  </h3>
                  <blockquote>{citation.excerpt}</blockquote>
                  <p>
                    Source revision:{" "}
                    {p.Guidance?.guidance?.sources?.[citation.source]?.commit}
                  </p>
                </div>
              ))}
            </details>
          )}
        </section>
      )}
      {p.EvidenceCoverage && (
        <EvidenceCoveragePanel
          service={service}
          coverage={p.EvidenceCoverage}
        />
      )}
      {p.Primary && (
        <section className="panel">
          <StatusBadge status={p.Primary.status} />
          <h2>{p.Primary.action}</h2>
          <p>{p.Primary.reason}</p>
          <ContextGaps gaps={p.Primary.context_gaps} />
          {p.Primary.next_steps?.map((step, index) => (
            <div
              className="next-step"
              key={`${step.title}-${step.explanation}-${step.targets?.join(
                ",",
              )}`}
            >
              <h3>
                {index + 1}. {step.title}
              </h3>
              <p>{step.explanation}</p>
              {step.targets && step.targets.length > 0 && (
                <details>
                  <summary>
                    {step.targets.length} related targets or sources
                  </summary>
                  <ul>
                    {step.targets.map((target) => (
                      <li key={target}>
                        <code>{target}</code>
                      </li>
                    ))}
                  </ul>
                </details>
              )}
              <p>{step.action}</p>
              {step.provider && (
                <Link
                  className="text-link"
                  to={`/services/${service}?page=Connections&provider=${encodeURIComponent(
                    step.provider,
                  )}`}
                >
                  Open {human(step.provider)} connections
                </Link>
              )}
            </div>
          ))}
          {p.Primary.status === "blocked" && (
            <p>
              After fixing access, rerun the affected check from{" "}
              <Link
                className="text-link"
                to={`/services/${service}?page=Activity`}
              >
                collection activity
              </Link>
              . Then submit any missing evidence and update the assessment
              before requesting human review.
            </p>
          )}
          <div className="legend">
            <Link
              className="text-link"
              to={`/services/${service}?page=Settings`}
            >
              Review scope and applicability
            </Link>
            <Link
              className="text-link"
              to={`/services/${service}?page=Sources`}
            >
              Open evidence sources
            </Link>
            <a className="text-link" href="#supporting-evidence">
              Inspect supporting evidence
            </a>
            {caps.includes("review") &&
              p.Forms?.some((f) => f.Action.endsWith("/review")) && (
                <Link
                  className="text-link"
                  to={`${location.pathname}?review=1#review`}
                >
                  Record human review
                </Link>
              )}
            <Link
              className="text-link"
              to={`/services/${service}?page=Connections`}
            >
              Review source access
            </Link>
          </div>
          <div className="status-gallery">
            {p.Primary.secondary?.map((label) => (
              <span className="badge" key={label}>
                {label}
              </span>
            ))}
          </div>
        </section>
      )}
      {!!p.Citations?.length && (
        <CitationList service={service} citations={p.Citations} />
      )}
      {p.Notice && <p className="panel muted">{p.Notice}</p>}
      {endpoint === "sources" && (
        <label className="input-label">
          Find a source
          <input
            value={params.get("q") || ""}
            onChange={(e) => {
              const q = new URLSearchParams(params);
              q.set("q", e.target.value);
              q.delete("source_offset");
              setParams(q);
            }}
            placeholder="Search names and locations"
          />
        </label>
      )}
      {data.error && (
        <>
          <p className="muted">
            Showing the last available data from{" "}
            {date(new Date(data.dataUpdatedAt).toISOString())}.
          </p>
          <ErrorPanel error={data.error} retry={() => data.refetch()} />
        </>
      )}
      {endpoint === "settings" && (
        <>
          <SetupChecklist service={service} page={p} />
          <AgentHandoff service={service} />
        </>
      )}{" "}
      {p.Detail && (
        <details className="panel">
          <summary>Agent investigation instructions</summary>
          <AgentHandoff
            service={service}
            requirement={endpoint.split("/")[1]}
          />
        </details>
      )}
      {!!p.Metrics?.length && (
        <div className="metrics">
          {p.Metrics.map((m) => (
            <div className="panel metric" key={m.Label}>
              <span className="muted">{m.Label}</span>
              <h2>{human(m.Value)}</h2>
            </div>
          ))}
        </div>
      )}
      {p.Sections?.map((s) => (
        <section className="panel" key={s.Title}>
          <h2>{s.Title}</h2>
          <Readable value={parseValue(s.Text)} />
        </section>
      ))}
      {!!p.Findings?.length && (
        <section className="panel">
          <h2>Findings to investigate</h2>
          {p.Findings.map((f) => (
            <div className="finding" key={f.Title}>
              <h3>{f.Title}</h3>
              <p>{f.Text}</p>
            </div>
          ))}
        </section>
      )}
      {p.Detail && (
        <section className="panel" id="supporting-evidence">
          <h2>Supporting evidence</h2>
          {!p.Evidence?.length && (
            <p className="muted">
              No supporting records yet. Ask an agent to investigate the missing
              assertions.
            </p>
          )}
          {p.Evidence?.map((e) => (
            <EvidenceCard key={e.Metadata} service={service} item={e} />
          ))}
          <div className="pagination">
            {p.Pagination?.map((l) => (
              <Link key={l.URL} className="text-link" to={l.URL}>
                {l.Text}
              </Link>
            ))}
          </div>
        </section>
      )}
      {p.Activities?.map((l) => (
        <Link className="panel attention" key={l.URL} to={l.URL}>
          {l.Text}
          <ArrowUpRight size={16} />
        </Link>
      ))}
      {p.Links?.filter(
        (l) =>
          (l.URL.includes("/exports/") && caps.includes("export")) ||
          l.URL.includes("batch="),
      ).map((l) => (
        <a className="panel attention" key={l.URL} href={l.URL}>
          {l.Text}
          <ArrowUpRight size={16} />
        </a>
      ))}
      {p.Forms?.filter((f) => caps.includes(permission(f.Action))).map((f) => (
        <details
          className="form-disclosure"
          key={
            endpoint +
            f.Action +
            f.Title +
            f.Fields.find((v) => v.Name === "id" || v.Name === "hash")?.Value
          }
          id={f.Action.endsWith("/review") ? "review" : undefined}
          open={
            f.Action.endsWith("/token") ||
            (!p.Detail && (p.Forms?.length || 0) < 5) ||
            (f.Action.endsWith("/review") &&
              location.search.includes("review=1"))
          }
        >
          <summary>
            {formTitle(f.Title, f.Fields)}
            {f.Hints?.map((hint) => (
              <p className="small muted" key={hint}>
                {hint}
              </p>
            ))}
          </summary>
          <ActionForm
            form={f}
            csrf={bootstrap.csrf}
            allowed={caps.includes(permission(f.Action))}
          />
        </details>
      ))}
      {!p.Detail && (
        <div className="pagination">
          {p.Pagination?.map((l) => (
            <Link className="text-link" key={l.URL} to={l.URL}>
              {l.Text}
            </Link>
          ))}
        </div>
      )}
      {!!p.Diagnostics?.length && (
        <details className="panel">
          <summary>Technical details and exact revisions</summary>
          {p.Diagnostics.map((s) => (
            <section key={s.Title}>
              <h3>{s.Title}</h3>
              <Readable value={parseValue(s.Text)} />
            </section>
          ))}
        </details>
      )}
    </>
  );
}
function formTitle(title: string, fields: { Name: string; Value: string }[]) {
  if (!title.startsWith("Source ·")) return title;
  try {
    const v = JSON.parse(
      fields.find((f) => f.Name === "payload")?.Value || "{}",
    );
    return v.type === "aws"
      ? `AWS account · ${v.name}`
      : v.type === "kubernetes"
        ? `Kubernetes · ${v.name}`
        : v.name || title;
  } catch {
    return title;
  }
}
function SetupChecklist({ service, page }: { service: string; page: Page }) {
  const configured = (action: string) =>
    Number(
      page.Forms?.find((f) => f.Action.endsWith(`/${action}`))?.Fields.find(
        (f) => f.Name === "expected_revision",
      )?.Value || 0,
    ) > 0;
  return (
    <section className="panel">
      <h2>Service setup</h2>
      <ol className="readable-list">
        <li>
          {configured("preview-profile")
            ? "✓ Profile selected"
            : "Choose a certification profile below"}
        </li>
        <li>
          {configured("scope")
            ? "✓ Assessment scope saved"
            : "Define the service boundary below"}
        </li>
        <li>
          <Link className="text-link" to={`/services/${service}?page=Sources`}>
            Configure and check evidence sources
          </Link>
        </li>
        <li>
          Issue scoped agent access, then copy the investigation instructions.
        </li>
      </ol>
    </section>
  );
}
function EvidenceCard({
  service,
  item,
}: {
  service: string;
  item: NonNullable<Page["Evidence"]>[number];
}) {
  const [open, setOpen] = useState(false);
  let record: RecordItem | undefined;
  try {
    record = JSON.parse(item.Metadata);
  } catch {}
  return (
    <article className="requirement-row">
      <div>
        <h3>{item.Title}</h3>
        <p className="muted">
          {item.Source} · {item.Observed}
        </p>
        <p className="small muted">{item.Target}</p>
      </div>
      {record && (
        <button type="button" onClick={() => setOpen(true)}>
          Inspect evidence
        </button>
      )}
      {open && record && (
        <EvidencePreview
          service={service}
          record={record}
          close={() => setOpen(false)}
        />
      )}
    </article>
  );
}
function AgentHandoff({
  service,
  requirement,
}: {
  service: string;
  requirement?: string;
}) {
  const [copied, setCopied] = useState(false);
  const prompt = `Connect to ${
    window.location.origin
  }/mcp using your scoped bearer token. For service ${service}${
    requirement ? `, requirement ${requirement}` : ""
  }, call list_evidence_sources, list_open_rules, and get_rule_instructions. Investigate using your own read-only tools and credentials within configured scope. Submit artifacts with submit_evidence_artifact and cited results with submit_rule_result. Report missing access and evidence honestly; retain exact source revisions.`;
  return (
    <section className="panel">
      <h2>Connect an external agent</h2>
      <p className="muted">
        Issue a scoped token below, then give your agent these instructions.
      </p>
      <details>
        <summary>Connection details and investigation prompt</summary>
        <p className="prose">{prompt}</p>
      </details>
      <button
        type="button"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(prompt);
            setCopied(true);
          } catch {
            setCopied(false);
          }
        }}
      >
        Copy agent instructions
      </button>
      {copied && <output>Copied.</output>}
    </section>
  );
}
