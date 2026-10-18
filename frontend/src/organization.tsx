import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { request } from "./api";
import { ErrorPanel } from "./forms";
import type { components } from "./generated";

type Organization = components["schemas"]["Organization"];
type Membership = components["schemas"]["Membership"];
type MemberPage = components["schemas"]["MemberPage"];

export function OrganizationPage({ csrf }: { csrf: string }) {
  const client = useQueryClient();
  const [offset, setOffset] = useState(0);
  const organization = useQuery({
    queryKey: ["organization"],
    queryFn: () => request<Organization>("/api/ui/v1/organization"),
  });
  const members = useQuery({
    queryKey: ["members", offset],
    queryFn: () => request<MemberPage>(`/api/ui/v1/members?offset=${offset}`),
  });
  const refresh = async () => {
    await Promise.all([
      client.invalidateQueries({ queryKey: ["bootstrap"] }),
      client.invalidateQueries({ queryKey: ["members"] }),
      client.invalidateQueries({ queryKey: ["organization"] }),
    ]);
  };
  return (
    <>
      <div className="page-heading">
        <div>
          <h1>Organization</h1>
          <p>Review access requests and manage your assessment team.</p>
        </div>
      </div>
      {organization.error && (
        <ErrorPanel
          error={organization.error}
          retry={() => organization.refetch()}
        />
      )}
      {organization.data && (
        <PolicyForm
          key={organization.data.revision}
          organization={organization.data}
          csrf={csrf}
          refresh={refresh}
        />
      )}
      <section className="panel">
        <h2>Users</h2>
        <p>
          Users sign in first and wait for approval. Approve a request with a
          role, or deny access. Viewers can read assessments and evidence; users
          can edit, collect, review and export; admins also manage users and
          settings.
        </p>
        <button type="button" onClick={() => members.refetch()}>
          Refresh users
        </button>
        {members.isPending && <output>Loading users…</output>}
        {members.error && (
          <ErrorPanel error={members.error} retry={() => members.refetch()} />
        )}
        {(members.data?.members || []).map((member) => (
          <MemberForm
            key={`${member.identity_id}-${member.revision}`}
            member={member}
            csrf={csrf}
            refresh={refresh}
          />
        ))}
        {members.data?.members?.length === 0 && <p>No users on this page.</p>}
        <nav aria-label="User pages">
          <button
            type="button"
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - 50))}
          >
            Previous users
          </button>{" "}
          <button
            type="button"
            disabled={members.data?.next_offset == null}
            onClick={() => setOffset(members.data?.next_offset ?? offset)}
          >
            Next users
          </button>
        </nav>
      </section>
    </>
  );
}

function PolicyForm({
  organization,
  csrf,
  refresh,
}: {
  organization: Organization;
  csrf: string;
  refresh: () => Promise<void>;
}) {
  const [name, setName] = useState(organization.name);
  const [domains, setDomains] = useState(
    (organization.allowed_domains || []).join(", "),
  );
  const [error, setError] = useState<Error>();
  const [busy, setBusy] = useState(false);
  return (
    <form
      className="panel organization-form"
      onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true);
        setError(undefined);
        try {
          await request("/api/ui/v1/organization", {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              "X-CSRF-Token": csrf,
            },
            body: JSON.stringify({
              name,
              allowed_domains: domains
                .split(",")
                .map((d) => d.trim())
                .filter(Boolean),
              expected_revision: organization.revision,
            }),
          });
          await refresh();
        } catch (e) {
          setError(
            e instanceof Error ? e : new Error("Organization update failed"),
          );
        } finally {
          setBusy(false);
        }
      }}
    >
      <h2>Organization settings</h2>
      <label>
        Organization name
        <input
          required
          maxLength={200}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </label>
      <label>
        Allowed email domains
        <input
          placeholder="example.com, subsidiary.com"
          value={domains}
          onChange={(e) => setDomains(e.target.value)}
          aria-describedby="domain-help"
        />
      </label>
      <p id="domain-help">
        Use exact domains separated by commas. Leaving this empty lets users of
        your configured sign-in provider request approval. Matching a domain
        never grants access automatically. Optional provider tenant restrictions
        are configured by the operator.
      </p>
      {error != null && <ErrorPanel error={error} retry={refresh} />}
      <button disabled={busy} type="submit">
        {busy ? "Saving…" : "Save organization"}
      </button>
    </form>
  );
}

function MemberForm({
  member,
  csrf,
  refresh,
}: {
  member: Membership;
  csrf: string;
  refresh: () => Promise<void>;
}) {
  const [role, setRole] = useState(member.role || "viewer");
  const [error, setError] = useState<Error>();
  const [busy, setBusy] = useState(false);
  const change = async (operation: string) => {
    setBusy(true);
    setError(undefined);
    try {
      await request("/api/ui/v1/members", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
        body: JSON.stringify({
          identity_id: member.identity_id,
          expected_revision: member.revision,
          operation,
          role,
        }),
      });
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e : new Error("Membership update failed"));
    } finally {
      setBusy(false);
    }
  };
  return (
    <section
      className="panel organization-form"
      aria-label={member.email || member.name}
    >
      <h3>{member.email || member.name}</h3>
      <p>
        {member.name} ·{" "}
        {member.state === "pending" ? "Waiting for approval" : member.state}{" "}
        {member.role ? `· ${member.role}` : ""}
      </p>
      {member.state !== "disabled" && (
        <label>
          Role for {member.email || member.name}
          <select
            value={role}
            onChange={(e) => setRole(e.target.value)}
            disabled={busy}
          >
            <option value="viewer">Viewer</option>
            <option value="user">User</option>
            <option value="admin">Admin</option>
          </select>
        </label>
      )}
      <div className="organization-actions">
        {member.state === "pending" && (
          <button
            type="button"
            disabled={busy}
            onClick={() => change("approve")}
          >
            Approve access
          </button>
        )}
        {member.state === "active" && (
          <button
            type="button"
            disabled={busy || role === member.role}
            onClick={() => change("role")}
          >
            Change role
          </button>
        )}
        {member.state !== "disabled" ? (
          <button
            type="button"
            disabled={busy}
            onClick={() => change("disable")}
          >
            {member.state === "pending" ? "Deny access" : "Disable access"}
          </button>
        ) : (
          <button
            type="button"
            disabled={busy}
            onClick={() => change("enable")}
          >
            Return to pending
          </button>
        )}
      </div>
      {error != null && <ErrorPanel error={error} retry={refresh} />}
    </section>
  );
}
