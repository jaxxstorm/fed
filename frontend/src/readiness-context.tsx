import { human } from "./api";
import type { components } from "./generated";

type Progress = components["schemas"]["ReadinessRequirementProgress"];

export function ContextGaps({
  gaps,
}: {
  gaps?: components["schemas"]["RequirementNextStep"][] | null;
}) {
  if (!gaps?.length) return null;
  return (
    <section aria-label="Evidence access and input gaps">
      <h3>Action Items</h3>
      <ul>
        {gaps.map((gap) => (
          <li key={`${gap.title}-${gap.explanation}-${gap.targets?.join(",")}`}>
            <strong>{gap.title}</strong>: {gap.explanation}
            {!!gap.targets?.length && (
              <details>
                <summary>
                  Affected targets ({new Set(gap.targets).size})
                </summary>
                <ul>
                  {[...new Set(gap.targets)].map((target) => (
                    <li key={target}>{target}</li>
                  ))}
                </ul>
              </details>
            )}
            <p>{gap.action}</p>
          </li>
        ))}
      </ul>
      <p>
        Unavailable evidence does not establish that implementation is absent.
      </p>
    </section>
  );
}

export function GuidanceState({
  state,
  reason,
}: {
  state?: string;
  reason?: string;
}) {
  if (!state || state === "reviewed") return null;
  const label =
    state === "missing"
      ? "Guidance missing"
      : state === "draft"
        ? "Draft guidance"
        : "Guidance needs revalidation";
  return (
    <p className="small muted">
      {label}. {reason} Official requirements and existing assessments remain
      available.
    </p>
  );
}

export function ScopedReadiness({
  progress,
  compact = false,
}: {
  progress?: Progress | null;
  compact?: boolean;
}) {
  if (!progress) return null;
  return (
    <section aria-label="Current scoped readiness">
      {!!progress.questions?.length && (
        <div>
          <h3>Unresolved conditions</h3>
          <ul>
            {progress.questions.map((q) => (
              <li
                key={`${q.fact}-${q.capability_id}-${q.question}-${JSON.stringify(q.scope)}`}
              >
                {q.question}{" "}
                {!compact && (
                  <p>
                    {q.reason} An unanswered condition does not establish
                    applicability.
                  </p>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
      {!compact &&
        progress.capabilities?.map((c) => (
          <article key={c.id}>
            <h3>
              {c.name} · {human(c.scope.kind)}
            </h3>
            <p>
              {[
                c.scope.account_id,
                c.scope.region,
                c.scope.cluster_id,
                c.scope.namespace_uid,
                c.scope.repository,
              ]
                .filter(Boolean)
                .join(" · ")}
            </p>
            <p>
              Implementation: {human(c.implementation)} · Evidence:{" "}
              {human(c.evidence_state)} · Review: {human(c.review)}
            </p>
            {c.declaration && (
              <p>
                Owner: {c.declaration.data.owner_role}
                {c.declaration.data.assignee_id &&
                  ` · ${c.declaration.data.assignee_id}`}{" "}
                · Declaration revision {c.declaration.revision}
              </p>
            )}
            <p>{c.next_action}</p>
            <p>{c.rationale}</p>
            <p>Dependent requirements: {c.requirement_ids?.join(", ")}</p>
            {c.evidence?.map((e) => (
              <p key={`${e.reference.id}-${e.reference.revision}`}>
                Candidate artifact {e.reference.id} revision{" "}
                {e.reference.revision}: {human(e.state)}. {e.reason}{" "}
                {e.limitations?.join(" ")}
              </p>
            ))}
          </article>
        ))}
      {!compact &&
        progress.duties?.map((d) => (
          <article key={d.id}>
            <h3>{d.step.action.text}</h3>
            <p>
              {d.step.owner_role} · Trigger: {human(d.trigger)} ·
              Implementation: {human(d.implementation)} · Evidence:{" "}
              {human(d.evidence_state)}
            </p>
            <p>{d.next_action}</p>
          </article>
        ))}
      {!compact && (
        <p>
          Saved declarations and candidate artifacts do not establish compliance
          or replace human review. Profile revision{" "}
          {progress.binding.profile_revision} · Scope revision{" "}
          {progress.binding.scope_revision} · Guidance revision{" "}
          {progress.binding.release_revision}.
        </p>
      )}
    </section>
  );
}
