import type { Status } from "./fixtures";
import type { components } from "./generated";

type MatchesContract<T extends U, U> = T;
export type BrowserContractCheck =
  | MatchesContract<Page, components["schemas"]["Page"]>
  | MatchesContract<Bootstrap, components["schemas"]["UIBootstrap"]>
  | MatchesContract<Overview, components["schemas"]["UIOverview"]>;

export interface Bootstrap {
  profiles?: Record<string, string>;
  services: { id: string; name: string; organization_id: string }[];
  csrf: string;
  capabilities: Record<string, string[]>;
  can_create: boolean;
}
export interface Field {
  Name: string;
  Label: string;
  Value: string;
  Type: string;
  Options: string[] | null;
}
export interface Form {
  References?:
    | { ID: string; Revision: number; Label: string; Kind: string }[]
    | null;
  Hints?: string[] | null;
  Action: string;
  Title: string;
  Button: string;
  CSRF: string;
  Fields: Field[];
}
export interface Page {
  Primary: Rule["state"] | null;
  Citations:
    | {
        Evidence: RecordItem;
        StartLine: number;
        EndLine: number;
        JSONPointer: string | null;
        Label: string;
      }[]
    | null;
  Rows: components["schemas"]["Page"]["Rows"];
  Title: string;
  Notice: string;
  Error: string;
  Forms: Form[] | null;
  Sections: { Title: string; Text: string }[] | null;
  Links: { Text: string; URL: string }[] | null;
  Detail: boolean;
  Metrics: { Label: string; Value: string }[] | null;
  Evidence:
    | {
        Title: string;
        Source: string;
        Observed: string;
        Target: string;
        URL: string;
        Action: string;
        Metadata: string;
      }[]
    | null;
  Findings: { Title: string; Text: string }[] | null;
  Diagnostics: { Title: string; Text: string }[] | null;
  Activities: { Text: string; URL: string }[] | null;
  Pagination: { Text: string; URL: string }[] | null;
}
export interface Counts {
  required: number;
  passing: number;
  evidence_complete: number;
  reviews_remaining: number;
  scope_decisions: number;
  optional: number;
  recommended: number;
  not_applicable: number;
  statuses: Record<string, number> | null;
}
export interface Rule {
  open_gap: boolean;
  id: string;
  name: string;
  category: string;
  kind: string;
  applicability: string;
  statement: string;
  state: {
    next_steps?: components["schemas"]["RequirementNextStep"][] | null;
    human_reviewed: boolean;
    status: Status;
    label: string;
    reason: string;
    action: string;
    evidence_complete: boolean;
    review_remaining: boolean;
    secondary: string[] | null;
  };
}
export interface Overview {
  evidence_records: number;
  as_of: string;
  rules: Rule[];
  counts: Counts;
  categories: Record<string, Counts>;
  setup_required: boolean;
}
export type Value =
  | null
  | boolean
  | string
  | number
  | Value[]
  | { [key: string]: Value };
export interface RecordItem {
  service_id: string;
  freshness?: string;
  linked_requirements?: string[];
  id: string;
  revision: number;
  kind: string;
  requirement_id: string;
  actor_id: string;
  created_at: string;
  data: Record<string, Value>;
}
export interface RecordPage {
  items: RecordItem[];
  has_more: boolean;
  offset: number;
}
export class APIError extends Error {
  constructor(
    public code: string,
    public title: string,
    public explanation: string,
    public reference = "",
    public status = 500,
    public fields: Record<string, string> = {},
  ) {
    super(explanation);
  }
}
export async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, { credentials: "same-origin", ...init });
  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new APIError(
      "unavailable",
      "The service is unavailable",
      "Try again in a moment.",
      "",
      response.status,
    );
  }
  if (!response.ok) {
    const e = body as Record<string, string>;
    throw new APIError(
      e.code || "unavailable",
      e.title || "Unable to load this page",
      e.explanation || "Try again in a moment.",
      e.reference,
      response.status,
      (body as { field_errors?: Record<string, string> }).field_errors || {},
    );
  }
  return body as T;
}
export const api = (service: string) =>
  `/api/ui/v1/services/${encodeURIComponent(service)}`;
export const human = (value: string) =>
  value.replaceAll("_", " ").replace(/\b\w/g, (c) => c.toUpperCase());
export const text = (value: Value | undefined) =>
  typeof value === "string"
    ? value
    : typeof value === "number"
      ? String(value)
      : "";
export const date = (value: string) =>
  new Date(value).toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
