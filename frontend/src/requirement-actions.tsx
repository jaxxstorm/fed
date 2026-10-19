import { human, type Page } from "./api";

export function RequirementActions({ page }: { page: Page }) {
  const book =
    page.Guidance?.state === "reviewed"
      ? page.Guidance.guidance?.playbooks?.[0]
      : undefined;
  const official = page.Sections?.find(
    (section) => section.Title === "What this requirement asks for",
  )?.Text;
  const excluded =
    book?.kind === "excluded" || page.Primary?.status === "not_applicable";
  const decision =
    book?.kind === "needs_decision" || page.Primary?.status === "needs_scoping";
  const setup = book?.kind === "actionable" ? book.setup.items || [] : [];
  const operate = book?.kind === "actionable" ? book.operate.items || [] : [];
  const actions = page.ActionItems || [];
  if (!book && !official && !actions.length) return null;
  return (
    <section
      className="requirement-actions"
      aria-labelledby="required-actions-heading"
    >
      <h2 id="required-actions-heading">Action Items</h2>
      {excluded && (
        <p>
          <strong>Not assigned by this profile.</strong>{" "}
          {book?.applicability.text || page.Primary?.reason} The work described
          below is context, not an additional obligation for this service.
        </p>
      )}
      {decision && (
        <p>
          <strong>Applicability needs a decision.</strong>{" "}
          {book?.decision_question || page.Primary?.action} Owner:{" "}
          {book?.decision_owner_role || "Service scope owner"}. The work below
          applies if this requirement is in scope.
        </p>
      )}
      {setup.length || operate.length ? (
        [
          { title: "Set up", steps: setup },
          { title: "Keep operating", steps: operate },
        ].map(
          ({ title, steps }) =>
            steps.length > 0 && (
              <section key={title}>
                <h3>{title}</h3>
                <ol>
                  {steps.map((step) => (
                    <li key={step.id}>
                      <p>
                        <strong>{step.action.text}</strong>
                      </p>
                      <p>
                        Owner: {step.owner_role} · {human(step.action.basis)}
                      </p>
                      <p>
                        When: {human(step.trigger)}
                        {step.timing ? ` · ${step.timing.text}` : ""}
                      </p>
                    </li>
                  ))}
                </ol>
              </section>
            ),
        )
      ) : actions.length > 0 ? (
        <ol>
          {actions.map((item) => (
            <li key={`${item.title}-${item.sources?.join("-")}`}>
              <h3>{item.title}</h3>
              <p>{item.action}</p>
              <p>
                <strong>Owner:</strong> {item.owner}
              </p>
              <p className="muted">
                {item.basis} · Source: {item.sources?.join(", ")}
              </p>
            </li>
          ))}
        </ol>
      ) : (
        <p className="requirement-actions-obligation">{official}</p>
      )}
      {!!page.ActionExpectations?.length && (
        <details>
          <summary>FedRAMP expectations and context</summary>
          <ul>
            {page.ActionExpectations.map((text) => (
              <li key={text}>{text}</li>
            ))}
          </ul>
        </details>
      )}
      {official && actions.length > 0 && (
        <details>
          <summary>Official requirement</summary>
          <p>{official}</p>
        </details>
      )}
      {book && page.Guidance?.caveat && (
        <p>
          {page.Guidance.label}. {page.Guidance.caveat}
        </p>
      )}
    </section>
  );
}
