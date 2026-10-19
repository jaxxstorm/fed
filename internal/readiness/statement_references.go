package readiness

// statementReferences deliberately excludes review findings: reviews bind the
// authored statements, and must not expand the source set they are reviewing.
func statementReferences(book Playbook, capabilities []Capability) map[string]bool {
	refs := map[string]bool{}
	statement := func(s Statement) {
		for _, id := range s.Citations {
			refs[id] = true
		}
	}
	steps := func(steps Steps) {
		for _, step := range steps.Items {
			statement(step.Action)
			if step.Timing != nil {
				statement(*step.Timing)
			}
			for _, s := range step.CompletionCriteria {
				statement(s)
			}
		}
	}
	dependencies := func(deps []Dependency) {
		for _, d := range deps {
			statement(d.Reason)
		}
	}
	statement(book.Purpose)
	statement(book.Applicability)
	dependencies(book.Prerequisites)
	steps(book.Setup)
	steps(book.Operate)
	steps(book.Collect)
	steps(book.ReviewSteps)
	for _, a := range book.Assertions {
		statement(a.Obligation)
		statement(a.Acceptance.Scope)
		statement(a.Acceptance.TimePeriod)
		statement(a.Acceptance.Provenance)
		for _, route := range a.Acceptance.Alternatives {
			for _, artifact := range route.Artifacts {
				statement(artifact)
			}
		}
	}
	for _, c := range capabilities {
		dependencies(c.Dependencies)
		steps(c.Setup)
		for _, alternative := range c.Alternatives {
			statement(alternative.Explanation)
			steps(alternative.Steps)
		}
	}
	return refs
}
