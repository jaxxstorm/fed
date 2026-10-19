import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api, human, request } from "./api";
import { ErrorPanel } from "./forms";
import type { components } from "./generated";

type Quality = components["schemas"]["ReadinessQualityPage"];

export function GuidanceQuality({ service }: { service: string }) {
  const [open, setOpen] = useState(false);
  const [offset, setOffset] = useState(0);
  const result = useQuery({
    queryKey: [service, "guidance-quality", offset],
    enabled: open,
    queryFn: () =>
      request<Quality>(
        `${api(service)}/readiness/coverage?limit=25&offset=${offset}`,
      ),
  });
  const quality = result.data;
  return (
    <details
      className="panel"
      onToggle={(event) => setOpen(event.currentTarget.open)}
    >
      <summary>Published guidance coverage</summary>
      <p>
        These counts measure guidance for all catalog variants, including
        excluded and unresolved variants. They are not service compliance
        counts. Draft authoring files do not count as published guidance.
      </p>
      {result.error && (
        <ErrorPanel error={result.error} retry={() => result.refetch()} />
      )}
      {open && result.isPending && <output>Loading guidance coverage…</output>}
      {quality && (
        <>
          <p>
            {quality.variant_count} {quality.count_unit} ·{" "}
            {quality.complete
              ? "Complete current reviewed coverage"
              : "Coverage gate not met"}
          </p>
          <ul>
            {Object.entries(quality.counts || {}).map(([state, count]) => (
              <li key={state}>
                {human(state)}: {count}
              </li>
            ))}
          </ul>
          <p>Published release: {quality.release_revision || "none"}</p>
          <ul>
            {quality.entries?.map((entry) => (
              <li key={entry.variant_id}>
                <strong>{entry.requirement_id}</strong> ·{" "}
                {human(entry.applicability)} · {human(entry.state)}
                {entry.reason && <p>{entry.reason}</p>}
                <details>
                  <summary>Variant identity</summary>
                  <p className="small">{entry.variant_id}</p>
                </details>
              </li>
            ))}
          </ul>
          <nav aria-label="Guidance coverage pages">
            <button
              type="button"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - 25))}
            >
              Previous variants
            </button>
            {quality.next_offset !== undefined && (
              <button
                type="button"
                onClick={() => setOffset(quality.next_offset as number)}
              >
                Next variants
              </button>
            )}
          </nav>
        </>
      )}
    </details>
  );
}
