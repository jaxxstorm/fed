import { useState } from "react";
import { api, date, human } from "./api";
import type { components } from "./generated";

export function EvidenceCoveragePanel({
  service,
  coverage,
}: {
  service: string;
  coverage: components["schemas"]["RequirementEvidenceContext"];
}) {
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  return (
    <section className="panel" aria-label="Evidence coverage">
      <h2>What the evidence demonstrates</h2>
      {coverage.limitations?.map((text) => (
        <p key={text}>{text}</p>
      ))}
      {coverage.assertions?.map((assertion) => (
        <article key={assertion.assertion_id}>
          <h3>{assertion.obligation}</h3>
          <p>{assertion.states?.map(human).join(" · ")}</p>
          <p>{assertion.rationale}</p>
          {!!assertion.gaps?.length && (
            <section aria-label="Uncovered scope and periods">
              <h4>What remains unsupported</h4>
              <ul>
                {assertion.gaps.map((gap) => (
                  <li key={JSON.stringify(gap)}>
                    <p>
                      {human(gap.kind)} · {gap.scope}
                      {gap.period ? ` · ${gap.period}` : ""}
                    </p>
                    <p>{gap.reason}</p>
                    <p>Next: {gap.next_action}</p>
                  </li>
                ))}
              </ul>
            </section>
          )}
          {assertion.acceptance && (
            <details>
              <summary>Acceptable proof and its limits</summary>
              <p>{assertion.acceptance.fact}</p>
              <p>Scope: {assertion.acceptance.scope.text}</p>
              <p>Time period: {assertion.acceptance.time_period.text}</p>
              <p>Provenance: {assertion.acceptance.provenance.text}</p>
              {assertion.acceptance.alternatives?.map((route) => (
                <div key={route.id}>
                  <h4>Evidence route: {route.id}</h4>
                  <ul>
                    {route.artifacts?.map((artifact) => (
                      <li key={artifact.text}>
                        {artifact.text} ({human(artifact.basis)})
                      </li>
                    ))}
                  </ul>
                  <p>Proves: {route.proves}</p>
                  <p>Does not prove: {route.limitations}</p>
                </div>
              ))}
              <h4>Insufficient on its own</h4>
              <ul>
                {assertion.acceptance.insufficient?.map((text) => (
                  <li key={text}>{text}</li>
                ))}
              </ul>
            </details>
          )}
          <ul>
            {assertion.citations?.map((c) => (
              <li
                key={`${c.evidence.id}-${c.evidence.revision}-${c.start_line}-${c.json_pointer}`}
              >
                <a
                  href={`${api(service)}/preview/${encodeURIComponent(c.evidence.id)}?revision=${c.evidence.revision}`}
                  target="_blank"
                  rel="noreferrer"
                >
                  Evidence {c.evidence.id} · revision {c.evidence.revision}
                </a>
                {c.json_pointer
                  ? ` · JSON ${c.json_pointer}`
                  : c.start_line
                    ? ` · lines ${c.start_line}–${c.end_line}`
                    : " · whole artifact"}
              </li>
            ))}
          </ul>
        </article>
      ))}
      <h3>Available artifacts</h3>
      {!coverage.candidates?.length && (
        <p>No linked artifacts have been collected for this requirement.</p>
      )}
      {coverage.candidates?.map((candidate) => (
        <article
          key={`${candidate.evidence.id}-${candidate.evidence.revision}`}
        >
          <h4>{candidate.description}</h4>
          <p>
            {human(candidate.state)} · observed {date(candidate.observed_at)}
          </p>
          <p>{candidate.reason}</p>
          {candidate.target && (
            <p>{Object.values(candidate.target).filter(Boolean).join(" · ")}</p>
          )}
          <a
            href={`${api(service)}/preview/${encodeURIComponent(candidate.evidence.id)}?revision=${candidate.evidence.revision}`}
            target="_blank"
            rel="noreferrer"
          >
            Open retained artifact · revision {candidate.evidence.revision}
          </a>
          {candidate.limitations?.map((text) => (
            <p key={text}>{text}</p>
          ))}
        </article>
      ))}
      <h3>Checks and findings</h3>
      <p>
        {coverage.collection.observed} of {coverage.collection.expected}{" "}
        expected observation targets covered ·{" "}
        {human(coverage.collection.technical || "unknown")}
        {coverage.collection.last_observed_at
          ? ` · last observed ${date(coverage.collection.last_observed_at)}`
          : ""}
      </p>
      {[
        ...(coverage.check_groups || []),
        ...(coverage.finding_groups || []),
      ].map((group) => (
        <article key={group.id}>
          <h4>
            {group.explanation.title} · {group.affected_targets} affected
            targets
          </h4>
          {group.outcome && (
            <p>
              Check result: {human(group.outcome)}
              {group.freshness ? ` · ${human(group.freshness)}` : ""}
              {group.last_observed_at
                ? ` · last observed ${date(group.last_observed_at)}`
                : ""}
            </p>
          )}
          <p>Expected: {group.explanation.expected}</p>
          <p>Observed: {group.explanation.observed}</p>
          <p>{group.explanation.limitations}</p>
          <p>
            {group.explanation.owner_role}: {group.explanation.next_action}
          </p>
          <details
            onToggle={(event) => {
              const open = event.currentTarget.open;
              setExpandedGroups((current) => {
                const next = new Set(current);
                if (open) next.add(group.id);
                else next.delete(group.id);
                return next;
              });
            }}
          >
            <summary>Resources and original observations</summary>
            {expandedGroups.has(group.id) &&
              group.findings?.map((f) => (
                <div key={`${f.id}-${f.observation_id}`}>
                  <p>
                    {Object.values(f.target).filter(Boolean).join(" · ")} ·{" "}
                    {date(f.observed_at)}
                  </p>
                  {f.observation_id && (
                    <a
                      href={`${api(service)}/observations/${encodeURIComponent(f.observation_id)}`}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open retained observation
                    </a>
                  )}
                  <p>
                    Diagnostic: {f.condition} · {f.outcome} · {f.basis}
                  </p>
                  <p>{f.explanation}</p>
                </div>
              ))}
          </details>
        </article>
      ))}
      {coverage.collection.recovery?.map((r) => (
        <p key={`${r.code}-${r.target}`}>
          {r.title}: {r.explanation}
        </p>
      ))}
      {!!coverage.blockers?.length && (
        <>
          <h3>Remaining inputs</h3>
          <ul>
            {coverage.blockers.map((text) => (
              <li key={text}>{text}</li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}
