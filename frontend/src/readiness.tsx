import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router-dom";
import { APIError, api, human, request } from "./api";
import { ErrorPanel } from "./forms";
import type { components } from "./generated";
import { GuidanceQuality } from "./guidance-quality";

type Plan = components["schemas"]["ReadinessServicePlan"];
type Capability = components["schemas"]["ReadinessCapabilityReadiness"];
type WorkData = components["schemas"]["ReadinessWorkData"];

function declarationKey() {
  // getRandomValues also works on HTTP development origins, unlike randomUUID.
  return Array.from(
    globalThis.crypto.getRandomValues(new Uint8Array(16)),
    (byte) => byte.toString(16).padStart(2, "0"),
  ).join("");
}

export function ReadinessPage({
  service,
  csrf,
  canEdit,
}: {
  service: string;
  csrf: string;
  canEdit: boolean;
}) {
  const [offset, setOffset] = useState(0);
  const result = useQuery({
    queryKey: [service, "readiness", offset],
    queryFn: () =>
      request<Plan>(`${api(service)}/readiness/plan?limit=25&offset=${offset}`),
  });
  if (result.error && !result.data)
    return <ErrorPanel error={result.error} retry={() => result.refetch()} />;
  if (!result.data)
    return <output className="panel">Loading readiness plan…</output>;
  const plan = result.data;
  return (
    <>
      <header className="page-heading">
        <h1>Service readiness</h1>
        <p>
          Shared prerequisites come first. Declarations and attached records do
          not establish compliance.
        </p>
      </header>
      {result.error && (
        <ErrorPanel error={result.error} retry={() => result.refetch()} />
      )}
      <p>
        {plan.total} scoped capability instances · {plan.question_count}{" "}
        unresolved questions
      </p>
      {plan.label && (
        <p>
          <strong>{plan.label}</strong>
        </p>
      )}
      {plan.caveat && <p>{plan.caveat}</p>}
      <GuidanceQuality service={service} />
      {plan.state !== "reviewed" && (
        <section className="panel">
          <h2>
            {plan.state === "missing"
              ? "Guidance is not published"
              : "Guidance needs source review"}
          </h2>
          <p>
            {plan.state === "missing"
              ? "The complete source-reviewed catalog has not been published for this service."
              : "Revalidate changed policy sources before relying on these instructions."}
          </p>
        </section>
      )}
      {plan.questions?.map((q) => (
        <section
          className="panel"
          key={`${q.requirement_id}-${q.capability_id}-${JSON.stringify(q.scope)}-${q.question}`}
        >
          <h2>{q.question}</h2>
          <p>{q.reason}</p>
          <Link to={`/services/${service}/requirements/${q.requirement_id}`}>
            {q.requirement_id}
          </Link>
          {canEdit &&
            plan.state === "reviewed" &&
            q.fact?.startsWith("decision.") &&
            q.scope &&
            q.capability_id && (
              <WorkForm
                service={service}
                csrf={csrf}
                binding={plan.binding}
                question={q}
                saved={() => result.refetch({ throwOnError: true })}
                node={{
                  id: `decision-${q.capability_id}-${JSON.stringify(q.scope)}`,
                  capability_id: q.capability_id,
                  name: q.capability_id,
                  scope: q.scope,
                  requirement_ids: [q.requirement_id],
                  prerequisite_ids: [],
                  implementation: "unknown",
                  evidence_state: "missing",
                  evidence: [],
                  trigger: "unknown",
                  review: "not_assessed",
                  blocked: true,
                  needs_revalidation: false,
                  steps: { items: [] },
                  alternatives: [],
                  next_action: q.question,
                  rationale: q.reason,
                  declaration: q.declaration,
                }}
              />
            )}
        </section>
      ))}
      {plan.capabilities?.map((n) => (
        <section className="panel" key={n.id} id={`capability-${n.id}`}>
          <h2>{n.name}</h2>
          <p>{Object.values(n.scope).filter(Boolean).join(" · ")}</p>
          <dl>
            <dt>Implementation</dt>
            <dd>{human(n.implementation)}</dd>
            <dt>Evidence</dt>
            <dd>{human(n.evidence_state)}</dd>
            <dt>Operating trigger</dt>
            <dd>{human(n.trigger)}</dd>
            <dt>Assessment</dt>
            <dd>{human(n.review)}</dd>
          </dl>
          {n.needs_revalidation && (
            <p>
              Saved declarations refer to older guidance or scope. Recheck them
              before reuse.
            </p>
          )}
          {n.blocked && (
            <p>
              Answer the prerequisite questions before treating these actions as
              applicable.
            </p>
          )}
          <h3>Next action</h3>
          <p>{n.next_action}</p>
          <p>{n.rationale}</p>
          <CandidateReferences service={service} items={n.evidence} />
          {!!n.prerequisite_ids?.length && (
            <p>
              First resolve {n.prerequisite_ids.length} prerequisite instances
              listed earlier in this plan.
            </p>
          )}
          <ol>
            {n.steps.items?.map((step) => (
              <li key={step.id}>
                <strong>{step.action.text}</strong>
                <p>
                  {human(step.action.basis)} · {step.owner_role}
                </p>
                {step.timing && <p>{step.timing.text}</p>}
                <ul>
                  {step.completion_criteria?.map((criterion) => (
                    <li key={`${step.id}-${criterion.text}`}>
                      {criterion.text}
                    </li>
                  ))}
                </ul>
              </li>
            ))}
          </ol>
          {n.steps.none_reason && <p>{n.steps.none_reason}</p>}
          <p>
            Used by{" "}
            {n.requirement_ids?.map((rid) => (
              <Link key={rid} to={`/services/${service}/requirements/${rid}`}>
                {rid}{" "}
              </Link>
            ))}
          </p>
          {canEdit && plan.state === "reviewed" && (
            <WorkForm
              service={service}
              csrf={csrf}
              node={n}
              binding={plan.binding}
              saved={() => result.refetch({ throwOnError: true })}
            />
          )}
        </section>
      ))}
      <section aria-label="Operating duties">
        <h2>Operating duties · {plan.duty_count || 0} duties</h2>
        <p>
          Shared prerequisites come first. Duties needing revalidation or
          reported due appear before other duties. Timing comes from cited
          guidance or labeled service choices; no fixed deadline is inferred.
        </p>
        {plan.duties?.map((duty) => (
          <article className="panel" key={duty.id}>
            <h3>{duty.step.action.text}</h3>
            <p>
              {duty.step.owner_role} · {human(duty.step.trigger)} ·{" "}
              {human(duty.step.action.basis)}
            </p>
            {duty.step.timing && (
              <p>
                Timing: {duty.step.timing.text} ({human(duty.step.timing.basis)}
                )
              </p>
            )}
            <dl>
              <dt>Implementation declaration</dt>
              <dd>{human(duty.implementation)}</dd>
              <dt>Operating trigger</dt>
              <dd>{human(duty.trigger)}</dd>
              <dt>Evidence</dt>
              <dd>{human(duty.evidence_state)}</dd>
              <dt>Assessment</dt>
              <dd>{human(duty.review)}</dd>
            </dl>
            <p>{duty.next_action}</p>
            <CandidateReferences service={service} items={duty.evidence} />
            <ul>
              {duty.step.completion_criteria?.map((criterion) => (
                <li key={criterion.text}>{criterion.text}</li>
              ))}
            </ul>
            <Link
              to={`/services/${service}/requirements/${duty.requirement_id}`}
            >
              {duty.requirement_id}
            </Link>
            {canEdit && plan.state === "reviewed" && (
              <WorkForm
                service={service}
                csrf={csrf}
                binding={plan.binding}
                saved={() => result.refetch({ throwOnError: true })}
                duty={duty}
                node={{
                  id: duty.id,
                  capability_id: "",
                  name: duty.step.action.text,
                  scope: { kind: "service" },
                  requirement_ids: [duty.requirement_id],
                  prerequisite_ids: [],
                  implementation: duty.implementation,
                  evidence_state: duty.evidence_state,
                  evidence: duty.evidence,
                  trigger: duty.trigger,
                  review: duty.review,
                  blocked: false,
                  needs_revalidation: duty.needs_revalidation,
                  steps: { items: [] },
                  alternatives: [],
                  declaration: duty.declaration,
                  next_action: duty.next_action,
                  rationale:
                    "Operating declarations do not establish completed duties.",
                }}
              />
            )}
          </article>
        ))}
      </section>
      <nav aria-label="Readiness pages">
        <button
          type="button"
          disabled={offset === 0}
          onClick={() => setOffset(Math.max(0, offset - 25))}
        >
          Previous
        </button>
        {plan.next_offset !== undefined && (
          <button
            type="button"
            onClick={() => setOffset(plan.next_offset as number)}
          >
            Next
          </button>
        )}
      </nav>
      <ReadinessRequirements service={service} />
    </>
  );
}

function ReadinessRequirements({ service }: { service: string }) {
  const [group, setGroup] = useState("evidence");
  const [cursors, setCursors] = useState<string[]>([""]);
  const cursor = cursors[cursors.length - 1];
  const result = useQuery({
    queryKey: [service, "readiness-requirements", group, cursor],
    queryFn: () =>
      request<components["schemas"]["ReadinessQueue"]>(
        `${api(service)}/readiness/requirements?limit=25&group=${group}&cursor=${encodeURIComponent(cursor)}`,
      ),
  });
  return (
    <section aria-label="Evidence and review work" className="panel">
      <h2>Evidence and human review</h2>
      <p>
        After prerequisites and operating duties, assess the retained evidence
        and arrange human review. These counts are requirements, separate from
        capability and duty counts.
      </p>
      <label>
        Work group{" "}
        <select
          value={group}
          onChange={(e) => {
            setGroup(e.target.value);
            setCursors([""]);
          }}
        >
          <option value="evidence">Evidence collection and assessment</option>
          <option value="review">Human review</option>
          <option value="scoping">Applicability decisions</option>
          <option value="maintain">Maintain reviewed evidence</option>
          <option value="excluded">Excluded requirements</option>
        </select>
      </label>
      {result.error && (
        <>
          <ErrorPanel error={result.error} retry={() => result.refetch()} />
          <button type="button" onClick={() => setCursors([""])}>
            Restart work list
          </button>
        </>
      )}
      {result.data && (
        <>
          <p>
            {result.data.total || 0} requirements in this group ·{" "}
            {result.data.counts?.evidence || 0} need evidence work ·{" "}
            {result.data.counts?.review || 0} await human review
          </p>
          <p>{result.data.ordering}</p>
          {result.data.items?.map((item) => (
            <article key={item.requirement_id}>
              <h3>
                <Link
                  to={`/services/${service}/requirements/${item.requirement_id}`}
                >
                  {item.name}
                </Link>
              </h3>
              <p>
                {item.presentation.label} · {human(item.applicability)}
              </p>
              <p>{item.presentation.reason}</p>
              <p>Next: {item.presentation.action}</p>
              {item.presentation.guidance_state &&
                item.presentation.guidance_state !== "reviewed" && (
                  <p>
                    Guidance: {human(item.presentation.guidance_state)}.
                    Source-reviewed instructions are not current for this
                    requirement.
                  </p>
                )}
              {item.presentation.review_remaining && (
                <p>
                  An authorized human must review the cited assessment. An agent
                  result or declaration cannot grant approval.
                </p>
              )}
            </article>
          ))}
          <nav aria-label="Requirement work pages">
            <button
              type="button"
              disabled={cursors.length === 1}
              onClick={() => setCursors(cursors.slice(0, -1))}
            >
              Previous requirements
            </button>
            <button
              type="button"
              disabled={!result.data.next_cursor}
              onClick={() =>
                setCursors([...cursors, result.data.next_cursor || ""])
              }
            >
              Next requirements
            </button>
          </nav>
        </>
      )}
    </section>
  );
}

function CandidateReferences({
  service,
  items,
}: {
  service: string;
  items: Capability["evidence"];
}) {
  if (!items?.length) return null;
  return (
    <details>
      <summary>Candidate artifacts and freshness</summary>
      <ul>
        {items.map((item) => (
          <li key={`${item.reference.id}-${item.reference.revision}`}>
            <a
              href={`${api(service)}/preview/${encodeURIComponent(item.reference.id)}?revision=${item.reference.revision}`}
              target="_blank"
              rel="noreferrer"
            >
              {item.reference.id} · revision {item.reference.revision}
            </a>
            <p>
              {human(item.state)}: {item.reason}
            </p>
            {item.limitations?.map((text) => (
              <p key={text}>{text}</p>
            ))}
          </li>
        ))}
      </ul>
    </details>
  );
}

function WorkForm({
  service,
  csrf,
  node,
  binding,
  saved,
  question,
  duty,
}: {
  service: string;
  csrf: string;
  node: Capability;
  binding: Plan["binding"];
  saved: () => Promise<unknown>;
  question?: components["schemas"]["ReadinessPlanQuestion"];
  duty?: components["schemas"]["ReadinessDutyReadiness"];
}) {
  const [data, setData] = useState<WorkData>(
    node.declaration?.data || {
      capability_id: node.capability_id,
      duty: duty?.duty,
      scope: node.scope,
      owner_role: duty?.step.owner_role || "",
      implementation: "unknown",
      trigger: "unknown",
      decisions: {},
      evidence: [],
      notes: "",
    },
  );
  const [revision, setRevision] = useState(node.declaration?.revision || 0);
  const [key, setKey] = useState(declarationKey);
  const [error, setError] = useState<Error | null>(null);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState("");
  const [compared, setCompared] = useState(false);
  const [evidenceID, setEvidenceID] = useState("");
  const [evidenceRevision, setEvidenceRevision] = useState(1);
  const update = (patch: Partial<WorkData>) => {
    setData({ ...data, ...patch });
    setKey(declarationKey());
    setMessage("");
  };
  const conflict =
    error instanceof APIError && error.code === "revision_conflict";
  return (
    <details>
      <summary>Record ownership or implementation declaration</summary>
      <form
        onSubmit={async (event) => {
          event.preventDefault();
          setPending(true);
          setError(null);
          try {
            const receipt = await request<
              components["schemas"]["ReadinessWorkRecord"]
            >(`${api(service)}/readiness/work`, {
              method: "POST",
              headers: {
                "Content-Type": "application/json",
                "X-CSRF-Token": csrf,
              },
              body: JSON.stringify({
                service_id: service,
                idempotency_key: key,
                expected_revision: revision,
                binding,
                data,
              }),
            });
            setRevision(receipt.revision);
            setKey(declarationKey());
            setMessage(
              "Declaration saved. Evidence assessment and human review remain separate.",
            );
            await saved();
          } catch (e) {
            setError(e as Error);
          } finally {
            setPending(false);
          }
        }}
      >
        {question?.fact && (
          <label>
            {question.question}
            <input
              required
              value={data.decisions?.[question.fact] || ""}
              placeholder={`Matching value: ${question.expected_value}`}
              onChange={(e) =>
                update({
                  decisions: {
                    ...data.decisions,
                    [question.fact as string]: e.target.value,
                  },
                })
              }
            />
            <span>
              Record the actual service fact. Leave it unanswered if unknown.
            </span>
          </label>
        )}
        <label>
          Responsible role
          <input
            required
            value={data.owner_role}
            onChange={(e) => update({ owner_role: e.target.value })}
          />
        </label>
        <label>
          Assigned identity ID (optional)
          <input
            value={data.assignee_id || ""}
            onChange={(e) => update({ assignee_id: e.target.value })}
          />
        </label>
        <label>
          Implementation declaration
          <select
            value={data.implementation}
            onChange={(e) => update({ implementation: e.target.value })}
          >
            <option value="unknown">Unknown</option>
            <option value="declared_missing">Declared missing</option>
            <option value="declared_present">Declared present</option>
          </select>
        </label>
        <label>
          Operating trigger
          <select
            value={data.trigger}
            onChange={(e) => update({ trigger: e.target.value })}
          >
            <option value="unknown">Unknown</option>
            {(!duty || duty.step.trigger === "event") && (
              <option value="not_triggered">Event has not occurred</option>
            )}
            <option value="due">Operating work is due</option>
            <option value="observed">
              Event occurred (not proof of completion)
            </option>
          </select>
        </label>
        {!!node.alternatives?.length && (
          <label>
            Implementation approach
            <select
              value={data.alternative_id || ""}
              onChange={(e) => update({ alternative_id: e.target.value })}
            >
              <option value="">Default approach</option>
              {node.alternatives.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.explanation.text}
                </option>
              ))}
            </select>
          </label>
        )}
        <label htmlFor={`${node.id}-notes`}>Notes</label>
        <textarea
          id={`${node.id}-notes`}
          value={data.notes}
          onChange={(e) => update({ notes: e.target.value })}
        />
        {!!data.evidence?.length && (
          <ul>
            {data.evidence.map((ref) => (
              <li key={`${ref.id}-${ref.revision}`}>
                Candidate evidence: {ref.id} revision {ref.revision}{" "}
                <button
                  type="button"
                  onClick={() =>
                    update({
                      evidence: data.evidence?.filter((r) => r !== ref) || [],
                    })
                  }
                >
                  Remove reference
                </button>
              </li>
            ))}
          </ul>
        )}
        <fieldset>
          <legend>Attach candidate evidence</legend>
          <p>
            This links an existing immutable record; it does not certify
            sufficiency.
          </p>
          <label>
            Evidence ID
            <input
              value={evidenceID}
              onChange={(e) => setEvidenceID(e.target.value)}
            />
          </label>
          <label>
            Evidence revision
            <input
              type="number"
              min="1"
              value={evidenceRevision}
              onChange={(e) => setEvidenceRevision(Number(e.target.value))}
            />
          </label>
          <button
            type="button"
            disabled={
              !evidenceID ||
              !Number.isInteger(evidenceRevision) ||
              evidenceRevision < 1
            }
            onClick={() => {
              if (
                !data.evidence?.some(
                  (r) => r.id === evidenceID && r.revision === evidenceRevision,
                )
              ) {
                update({
                  evidence: [
                    ...(data.evidence || []),
                    { id: evidenceID, revision: evidenceRevision },
                  ],
                });
              }
              setEvidenceID("");
            }}
          >
            Attach reference
          </button>
        </fieldset>
        {error && <ErrorPanel error={error} />}
        {conflict && (
          <>
            <button
              type="button"
              onClick={async () => {
                try {
                  await saved();
                  setCompared(true);
                } catch (e) {
                  setError(e as Error);
                }
              }}
            >
              Load current declaration for comparison
            </button>
            {compared && (
              <>
                <pre>{JSON.stringify(node.declaration?.data, null, 2)}</pre>
                <button
                  type="button"
                  onClick={() => {
                    setRevision(node.declaration?.revision || 0);
                    setKey(declarationKey());
                    setError(null);
                    setCompared(false);
                  }}
                >
                  I compared the current declaration; keep my edits
                </button>
              </>
            )}
          </>
        )}
        <button type="submit" disabled={pending || conflict}>
          {pending ? "Saving…" : "Save declaration"}
        </button>
        {message && <output>{message}</output>}
      </form>
    </details>
  );
}
