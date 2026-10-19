import { human, type Page } from "./api";

export function RequirementActions({ page }: { page: Page }) {
  const book =
    page.Guidance?.state === "reviewed"
      ? page.Guidance.guidance?.playbooks?.[0]
      : undefined;
  const official = page.Sections?.find(
    (section) => section.Title === "What this requirement asks for",
  )?.Text;
  const obligations = official
    ? [official]
    : page.EvidenceCoverage?.assertions?.map(
        (assertion) => assertion.obligation,
      ) || [];
  if (!book && !obligations.length) return null;
  const excluded =
    book?.kind === "excluded" || page.Primary?.status === "not_applicable";
  const decision =
    book?.kind === "needs_decision" || page.Primary?.status === "needs_scoping";
  const steps = book?.kind === "actionable" ? book.collect.items || [] : [];
  return (
    <section
      className="requirement-actions"
      aria-labelledby="required-actions-heading"
    >
      <p className="requirement-actions-eyebrow">Your action plan</p>
      <h2 id="required-actions-heading">This is what you need to do</h2>
      {excluded ? (
        <>
          <h3>Confirm the exclusion</h3>
          <p>{book?.applicability.text || page.Primary?.reason}</p>
          <p>
            No implementation or evidence checklist is assigned by this excluded
            requirement.
          </p>
        </>
      ) : decision ? (
        <>
          <h3>Resolve applicability first</h3>
          <p>{book?.decision_question || page.Primary?.action}</p>
          <p>
            <strong>Who:</strong>{" "}
            {book?.decision_owner_role || "Service scope owner"}
          </p>
        </>
      ) : (
        <>
          <h3>What is required</h3>
          {book && <p>{book.purpose.text}</p>}
          {obligations.map((text) => (
            <p className="requirement-actions-obligation" key={text}>
              {text}
            </p>
          ))}
          <h3>Your next evidence action</h3>
          {steps.length ? (
            <ol>
              {steps.map((step) => (
                <li key={step.id}>
                  <p>{step.action.text}</p>
                  <p>
                    <strong>Who:</strong> {step.owner_role} ·{" "}
                    {human(step.action.basis)}
                  </p>
                </li>
              ))}
            </ol>
          ) : (
            <>
              <p>
                Ask the party named in the requirement for records showing that
                the stated duty was completed for the systems or service in
                scope. Retain the records for assessment.
              </p>
              <p>
                <strong>Who:</strong> The party named in the requirement—not
                automatically the cloud provider.
              </p>
              <p>
                Requirement-specific evidence instructions are not available in
                this view yet.
              </p>
            </>
          )}
          <p className="requirement-actions-note">
            Collect evidence first. Missing records do not mean the work has not
            been done. Evidence assessment and human review are still required.
          </p>
        </>
      )}
      {book && page.Guidance?.caveat && (
        <p>
          {page.Guidance.label}. {page.Guidance.caveat}
        </p>
      )}
    </section>
  );
}
