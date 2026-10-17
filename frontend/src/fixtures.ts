export type Status =
  | "passing"
  | "failing"
  | "blocked"
  | "needs_refresh"
  | "needs_evidence"
  | "in_review"
  | "in_progress"
  | "not_started"
  | "needs_scoping"
  | "not_applicable";
export const statusLabels: Record<Status, string> = {
  passing: "Passing",
  failing: "Failing",
  blocked: "Blocked",
  needs_refresh: "Needs refresh",
  needs_evidence: "Needs evidence",
  in_review: "In review",
  in_progress: "In progress",
  not_started: "Not started",
  needs_scoping: "Needs scoping",
  not_applicable: "Not applicable",
};
export const requirements: {
  id: string;
  title: string;
  status: Status;
  next: string;
}[] = [
  {
    id: "KSI-SVC-ACM",
    title: "Automating configuration management",
    status: "needs_evidence",
    next: "Add dated drift-review results",
  },
  {
    id: "KSI-RPL-RRO",
    title: "Recovery objectives",
    status: "blocked",
    next: "Provide approved retention objectives",
  },
  {
    id: "fixture-encryption",
    title: "Protecting stored information",
    status: "failing",
    next: "Review 4 resource findings",
  },
  {
    id: "fixture-review",
    title: "Reviewing deployment changes",
    status: "in_review",
    next: "Review cited deployment records",
  },
];
export const evidence = Array.from({ length: 284 }, (_, index) => ({
  id: `fixture-${index}`,
  title:
    index === 0
      ? "Deployment workflow and pinned infrastructure revision"
      : `Operational review record ${index + 1}`,
  source: index % 2 ? "Operations records" : "Infrastructure repository",
  freshness: index % 7 === 0 ? "Needs refresh" : "Current",
  observed: "Oct 12, 2026",
}));
