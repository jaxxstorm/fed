import { Link } from "react-router-dom";
import { human } from "./api";
import type { components } from "./generated";

type Playbook = components["schemas"]["ReadinessPlaybook"];
type Steps = components["schemas"]["ReadinessSteps"];
type Package = components["schemas"]["ReadinessPackage"];

function ActionSteps({ title, steps }: { title: string; steps?: Steps }) {
  if (!steps) return null;
  return (
    <section>
      <h3>{title}</h3>
      {steps.none_reason && <p>{steps.none_reason}</p>}
      <ol>
        {steps.items?.map((step) => (
          <li key={step.id}>
            <p>
              <strong>{step.action.text}</strong>
            </p>
            <p>
              {step.owner_role} · {human(step.action.basis)} ·{" "}
              {human(step.trigger)}
            </p>
            {step.timing && (
              <p>
                Timing: {step.timing.text} ({human(step.timing.basis)})
              </p>
            )}
            {step.trigger === "event" && (
              <p>
                Establish whether the event occurred before treating this duty
                as due. No event is not proof of completed operation.
              </p>
            )}
            <ul>
              {step.completion_criteria?.map((criterion) => (
                <li key={criterion.text}>
                  {criterion.text} ({human(criterion.basis)})
                </li>
              ))}
            </ul>
            {!!step.action.citations?.length && (
              <p>Source references: {step.action.citations.join(", ")}</p>
            )}
          </li>
        ))}
      </ol>
    </section>
  );
}

export function PlaybookActions({
  service,
  book,
  guidance,
}: {
  service: string;
  book: Playbook;
  guidance: Package;
}) {
  if (book.kind === "excluded")
    return (
      <p>This variant is excluded; no implementation checklist is implied.</p>
    );
  if (book.kind === "needs_decision")
    return (
      <section>
        <h3>Applicability question</h3>
        <p>
          {book.decision_owner_role}: {book.decision_question}
        </p>
      </section>
    );
  if (book.kind !== "actionable") return null;
  return (
    <>
      <section>
        <h3>What must exist first</h3>
        {book.no_prerequisites_reason && <p>{book.no_prerequisites_reason}</p>}
        {book.prerequisites?.map((dependency) => {
          const capability = guidance.capabilities?.find(
            (c) => c.id === dependency.capability_id,
          );
          return (
            <article key={dependency.capability_id}>
              <h4>{capability?.name || dependency.capability_id}</h4>
              <p>
                {dependency.reason.text} ({human(dependency.reason.basis)})
              </p>
              {dependency.condition && (
                <p>
                  Conditional: {dependency.condition.question} Applies when{" "}
                  {dependency.condition.fact} is “{dependency.condition.equals}
                  ”. An unanswered question does not establish applicability.
                </p>
              )}
              {capability && (
                <>
                  <p>
                    Reuse only within the same{" "}
                    {human(capability.scope_kind).toLowerCase()} boundary.
                  </p>
                  <ActionSteps
                    title="Prerequisite setup"
                    steps={capability.setup}
                  />
                  {capability.alternatives?.map((alternative) => (
                    <details key={alternative.id}>
                      <summary>
                        Alternative: {alternative.explanation.text}
                      </summary>
                      <ActionSteps
                        title="Alternative steps"
                        steps={alternative.steps}
                      />
                    </details>
                  ))}
                </>
              )}
            </article>
          );
        })}
        <Link to={`/services/${service}?page=Readiness`}>
          Check shared prerequisites and saved declarations
        </Link>
      </section>
      <ActionSteps title="Setup actions" steps={book.setup} />
      <ActionSteps title="Operating duties" steps={book.operate} />
      <ActionSteps title="Evidence to collect" steps={book.collect} />
      <ActionSteps
        title="Ready-for-review criteria"
        steps={book.review_steps}
      />
    </>
  );
}
