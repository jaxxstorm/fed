import type { Page } from "./api";

function unique<T>(items: T[]): T[] {
  return [
    ...new Map(items.map((item) => [JSON.stringify(item), item])).values(),
  ];
}

export function ImplementationAssessmentPanel({
  context,
}: {
  context: NonNullable<Page["Implementation"]>;
}) {
  const assessment = context.assessment;
  const fields = assessment ? Object.entries(assessment.narrative) : [];
  return (
    <section className="panel" aria-label="Implementation assessment">
      <h2>How this service implements the requirement</h2>
      <p>
        {context.readiness.ready
          ? "Structurally ready for human review"
          : "Assessment needs more work"}
        . This checks completeness, not whether the evidence proves the claims.
      </p>
      <p>
        Current evidence freshness: {context.current_freshness}. Human review:{" "}
        {context.review}.
      </p>
      {unique(context.readiness.blockers ?? []).map((b) => (
        <p key={JSON.stringify(b)}>
          {b.explanation} {b.next_action}
        </p>
      ))}
      {assessment && (
        <>
          <p>
            Assessed period: {assessment.period.start} – {assessment.period.end}
            . Historical support does not establish operations outside this
            period.
          </p>
          <dl>
            {fields.map(([name, field]) => (
              <div key={name}>
                <dt>{name.replaceAll("_", " ")}</dt>
                <dd>
                  {field.state === "known"
                    ? field.text
                    : `${field.state.replaceAll("_", " ")}: ${field.reason}`}
                </dd>
              </div>
            ))}
          </dl>
          {assessment.assertions?.map((assertion) => (
            <details key={assertion.assertion_id}>
              <summary>{assertion.claim}</summary>
              <p>
                Implementation: {assertion.implementation.outcome}.{" "}
                {assertion.implementation.rationale}
              </p>
              <p>
                Operating evidence: {assertion.operating.outcome}.{" "}
                {assertion.operating.rationale}
              </p>
              <p>
                Sample: {assertion.sample.sample_size} /{" "}
                {assertion.sample.population ?? "unknown population"}.{" "}
                {assertion.sample.method} {assertion.sample.coverage_reason}{" "}
                {assertion.sample.population_unknown_reason}
              </p>
              {unique([
                ...(assertion.implementation.limitations ?? []),
                ...(assertion.operating.limitations ?? []),
              ]).map((l) => (
                <p key={l}>{l}</p>
              ))}
              <h3>Exact evidence references</h3>
              {unique([
                ...(assertion.implementation.citations ?? []),
                ...(assertion.operating.citations ?? []),
              ]).map((c) => (
                <p key={JSON.stringify(c)}>
                  {c.evidence.id} · revision {c.evidence.revision} ·{" "}
                  {c.json_pointer !== undefined
                    ? `JSON pointer ${c.json_pointer}`
                    : `lines ${c.start_line}–${c.end_line}`}
                </p>
              ))}
              {assertion.sample.events?.map((event) => (
                <p key={JSON.stringify(event)}>
                  {event.occurred_at}: {event.description}
                </p>
              ))}
            </details>
          ))}
        </>
      )}
    </section>
  );
}
