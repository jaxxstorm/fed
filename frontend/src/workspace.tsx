import * as Dialog from "@radix-ui/react-dialog";
import {
  ArrowUpRight,
  CheckCircle2,
  Circle,
  CircleAlert,
  Clock3,
  Database,
  Download,
  FileCheck2,
  LayoutDashboard,
  ListChecks,
  Menu,
  Radio,
  Settings,
  ShieldCheck,
  X,
} from "lucide-react";
import { type ReactNode, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import type { Value } from "./api";
import { evidence, requirements, type Status, statusLabels } from "./fixtures";
import { Readable, StructuredInput } from "./forms";

const navigation = [
  ["Overview", LayoutDashboard],
  ["Requirements", ListChecks],
  ["Evidence", FileCheck2],
  ["Activity", Radio],
  ["Sources", Database],
  ["Settings", Settings],
  ["Exports", Download],
] as const;
export function StatusBadge({ status }: { status: Status }) {
  const Icon =
    status === "passing"
      ? CheckCircle2
      : status === "failing" || status === "blocked"
        ? CircleAlert
        : status === "not_started"
          ? Circle
          : Clock3;
  return (
    <span className={`badge ${status}`}>
      <Icon size={14} aria-hidden="true" />
      {statusLabels[status]}
    </span>
  );
}
function Navigation({ close }: { close?: () => void }) {
  const [params] = useSearchParams();
  return (
    <>
      <div className="brand">
        <ShieldCheck size={27} />
        <span>
          Assessment<span className="muted brand-sub">Evidence workspace</span>
        </span>
      </div>
      <div className="service-card">
        <strong>Example service</strong>
        <span className="muted">Visual fixture · Class B</span>
      </div>
      <nav aria-label="Main navigation">
        {navigation.map(([label, Icon]) => (
          <Link
            key={label}
            onClick={close}
            className={
              (params.get("page") || "Overview") === label
                ? "nav-link selected"
                : "nav-link"
            }
            to={`?page=${label}`}
            aria-current={
              (params.get("page") || "Overview") === label ? "page" : undefined
            }
          >
            <Icon size={19} />
            {label}
          </Link>
        ))}
      </nav>
      <div className="sidebar-footer">
        Design preview
        <br />
        <span className="muted">Illustrative data only</span>
      </div>
    </>
  );
}
export function Shell({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="app">
      <a className="skip" href="#main">
        Skip to content
      </a>
      <aside className="sidebar">
        <Navigation />
      </aside>
      <div className="workspace">
        <header className="topbar">
          <Dialog.Root open={open} onOpenChange={setOpen}>
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
                  Choose a workspace page
                </Dialog.Description>
                <Dialog.Close
                  className="icon-button close"
                  aria-label="Close navigation"
                >
                  <X />
                </Dialog.Close>
                <Navigation close={() => setOpen(false)} />
              </Dialog.Content>
            </Dialog.Portal>
          </Dialog.Root>
          <span>
            Example service <span className="muted">/ Workspace preview</span>
          </span>
          <span className="top-label">Fixture gallery</span>
        </header>
        <main id="main" tabIndex={-1}>
          {children}
        </main>
      </div>
    </div>
  );
}
function Heading({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="page-heading">
      <div>
        <p className="eyebrow">ASSESSMENT WORKSPACE</p>
        <h1>{title}</h1>
        <p className="muted">{description}</p>
      </div>
      <Link className="button" to="?page=Sources">
        Explore sources <ArrowUpRight size={16} />
      </Link>
    </div>
  );
}
function Overview() {
  return (
    <>
      <Heading
        title="See where you stand."
        description="A clear view of your progress, evidence, and next steps."
      />
      <div className="metrics">
        {[
          ["Passing requirements", "24", "115", "passing"],
          ["Evidence gathered", "63", "115", "in_progress"],
          ["Reviews remaining", "39", "", "in_review"],
        ].map(([label, value, total, status]) => (
          <Link
            to={`?page=Requirements&status=${status}`}
            className="panel metric"
            key={label}
          >
            <span className="muted">{label}</span>
            <div className="metric-value">
              {value}
              {total && <span> / {total}</span>}
            </div>
            <span className="metric-caption">
              {total
                ? "Applicable required requirements"
                : "Requirements ready for review"}
              <ArrowUpRight size={15} />
            </span>
          </Link>
        ))}
      </div>
      <div className="panel progress-panel">
        <div className="section-heading">
          <h2>Assessment progress</h2>
          <strong>21% passing</strong>
        </div>
        <div
          className="progress-track"
          role="img"
          aria-label="24 of 115 required requirements passing; 39 in review; 52 need more work"
        >
          <span style={{ width: "21%" }} />
          <span style={{ width: "34%" }} />
          <span style={{ width: "45%" }} />
        </div>
        <div className="legend">
          <span>● 24 passing</span>
          <span>● 39 in review</span>
          <span>● 52 need more work</span>
        </div>
      </div>
      <div className="columns">
        <section className="panel">
          <div className="section-heading">
            <h2>What needs attention</h2>
            <span className="muted">Next steps</span>
          </div>
          {requirements.slice(0, 3).map((r) => (
            <Link
              className="attention"
              key={r.id}
              to={`?page=Requirements&status=${r.status}`}
            >
              <CircleAlert size={20} />
              <div>
                <strong>{r.next}</strong>
                <p className="muted">{r.title}</p>
              </div>
              <ArrowUpRight size={17} />
            </Link>
          ))}
        </section>
        <section className="panel">
          <div className="section-heading">
            <h2>Latest agent results</h2>
          </div>
          <div className="activity">
            <span className="activity-dot" />
            <div>
              <strong>Deployment workflow added</strong>
              <p className="muted">
                Configuration automation documented. Drift-review results are
                still needed.
              </p>
              <span className="small muted">
                Example result · 12 minutes ago
              </span>
            </div>
          </div>
          <Link className="text-link" to="?page=Activity">
            View activity <ArrowUpRight size={15} />
          </Link>
        </section>
      </div>
    </>
  );
}
function Requirements() {
  const [params, setParams] = useSearchParams();
  const filter = params.get("status") || "";
  return (
    <>
      <Heading
        title="Requirements"
        description="Understand each outcome and what it takes to finish."
      />
      <div className="panel">
        <label className="filter">
          Status
          <select
            value={filter}
            onChange={(e) =>
              setParams({
                page: "Requirements",
                ...(e.target.value ? { status: e.target.value } : {}),
              })
            }
          >
            <option value="">All statuses</option>
            {Object.entries(statusLabels).map(([key, label]) => (
              <option key={key} value={key}>
                {label}
              </option>
            ))}
          </select>
        </label>
        {requirements
          .filter((r) => !filter || r.status === filter)
          .map((r) => (
            <article className="requirement-row" key={r.id}>
              <div>
                <span className="small muted">{r.id}</span>
                <h2>{r.title}</h2>
                <p className="muted">{r.next}</p>
              </div>
              <StatusBadge status={r.status} />
            </article>
          ))}
        {!requirements.some((r) => !filter || r.status === filter) && (
          <div className="empty">
            <h2>No matching requirements</h2>
            <p>Try another status or clear the filter.</p>
            <button
              type="button"
              onClick={() => setParams({ page: "Requirements" })}
            >
              Clear filter
            </button>
          </div>
        )}
      </div>
    </>
  );
}
function Evidence() {
  const [page, setPage] = useState(0);
  return (
    <>
      <Heading
        title="Evidence"
        description="Explore what was gathered and what needs updating."
      />
      <div className="panel">
        <div className="section-heading">
          <h2>{evidence.length} evidence records</h2>
          <span className="muted">Example dataset</span>
        </div>
        {evidence.slice(page * 25, (page + 1) * 25).map((e) => (
          <article className="requirement-row" key={e.id}>
            <div>
              <h3>{e.title}</h3>
              <p className="muted">
                {e.source} · {e.observed}
              </p>
            </div>
            <span className="badge">{e.freshness}</span>
          </article>
        ))}
        <div className="pagination">
          <button
            type="button"
            disabled={page === 0}
            onClick={() => setPage(page - 1)}
          >
            Previous
          </button>
          <span aria-live="polite">
            Page {page + 1} of {Math.ceil(evidence.length / 25)}
          </span>
          <button
            type="button"
            disabled={(page + 1) * 25 >= evidence.length}
            onClick={() => setPage(page + 1)}
          >
            Next
          </button>
        </div>
      </div>
    </>
  );
}
function States() {
  const [source, setSource] = useState<Value>({
    type: "repository",
    name: "Deployment evidence",
    repository: "example/production-infrastructure",
    commit: "188f51f38ebf13869d1821c10a33e764aaa7fdbc",
    paths: [".github/workflows/deploy.yml"],
    freshness_hours: 48,
    enabled: true,
  });
  return (
    <>
      <Heading
        title="Feedback and recovery"
        description="Clear explanations, useful next steps, and honest states."
      />
      <div className="columns">
        <section className="panel">
          <h2>Configure an evidence source</h2>
          <StructuredInput name="Source" value={source} onChange={setSource} />
        </section>
        <section className="panel">
          <h2>Evidence preview</h2>
          <p className="muted">Pinned configuration · Lines 1–3</p>
          <div className="artifact-reader">
            <ol>
              <li>
                <code>name: Deployment</code>
              </li>
              <li>
                <code>on: pull_request</code>
              </li>
              <li>
                <code>permissions: read-all</code>
              </li>
            </ol>
          </div>
          <Readable
            value={{
              source: "Infrastructure repository",
              revision: 1,
              linked_requirements: 2,
            }}
          />
        </section>
      </div>
      <div className="panel">
        <h2>Requirement states</h2>
        <div className="status-gallery">
          {Object.keys(statusLabels).map((status) => (
            <StatusBadge key={status} status={status as Status} />
          ))}
        </div>
      </div>
      <section className="panel error-panel">
        <CircleAlert />
        <div>
          <h2>Cluster access denied</h2>
          <p>
            The example agent could not list namespaces in the production
            cluster.
          </p>
          <p className="muted">
            Refresh the agent’s Kubernetes access, then ask it to repeat the
            read-only investigation.
          </p>
          <Link className="text-link" to="?page=Sources">
            Open source settings <ArrowUpRight size={15} />
          </Link>
        </div>
      </section>
      <div className="panel empty">
        <h2>No sources checked yet</h2>
        <p className="muted">
          Configure a source and ask an agent to gather evidence.
        </p>
      </div>
      <output className="panel loading-state">
        <p>Refreshing evidence…</p>
        <div className="skeleton" />
      </output>
      <div className="panel">
        <strong>Showing the last available results</strong>
        <p className="muted">
          The latest refresh could not be completed. Last updated 10 minutes
          ago.
        </p>
      </div>
    </>
  );
}
export function Gallery() {
  const [params] = useSearchParams();
  const page = params.get("page") || "Overview";
  return (
    <Shell>
      <div className="preview-notice">
        Design preview — all counts and records on this page are illustrative.
      </div>
      {page === "Overview" ? (
        <Overview />
      ) : page === "Requirements" ? (
        <Requirements />
      ) : page === "Evidence" ? (
        <Evidence />
      ) : (
        <States />
      )}
    </Shell>
  );
}
