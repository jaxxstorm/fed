import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { api, human, request } from "./api";
import { ErrorPanel } from "./forms";
import type { components } from "./generated";

type Schema = components["schemas"];
type Setup = Schema["Setup"];
type Material = Schema["SetupMaterial"];

function requestID() {
  return Array.from(
    globalThis.crypto.getRandomValues(new Uint8Array(16)),
    (byte) => byte.toString(16).padStart(2, "0"),
  ).join("");
}

export function ProviderSetup({
  service,
  provider,
  csrf,
  canConnect,
}: {
  service: string;
  provider: string;
  csrf: string;
  canConnect: boolean;
}) {
  const [params, setParams] = useSearchParams();
  const cache = useQueryClient();
  const setupID = params.get("setup");
  const root = `${api(service)}/connections`;
  const [name, setName] = useState("");
  const [account, setAccount] = useState("");
  const [role, setRole] = useState("");
  const [method, setMethod] = useState("cli");
  const [material, setMaterial] = useState<Material | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [notice, setNotice] = useState("");
  const loaded = useRef<string | null>(null);
  const confirmation = useRef<HTMLHeadingElement | null>(null);
  const readiness = useQuery({
    queryKey: [service, "setup-readiness", provider],
    queryFn: () =>
      request<Schema["SetupReadiness"]>(`${root}/readiness/${provider}/status`),
  });
  const progress = useQuery({
    queryKey: [service, "setup", setupID],
    enabled: !!setupID,
    queryFn: () =>
      request<Setup>(
        `${root}/setups/${encodeURIComponent(setupID || "")}/status`,
      ),
  });
  const setup = progress.data;
  useEffect(() => {
    if (setup?.step === "confirm") confirmation.current?.focus();
  }, [setup?.step]);
  useEffect(() => {
    if (setup && loaded.current !== setup.id) {
      setName(setup.input.name);
      setAccount(setup.input.account_id || "");
      setRole(setup.input.role_arn || "");
      loaded.current = setup.id;
    }
  }, [setup]);
  const terminal =
    !!setup && ["complete", "expired", "cancelled"].includes(setup.step);
  async function post<T>(action: string, body: object): Promise<T> {
    return request<T>(`${root}/setups/${action}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
      body: JSON.stringify({ ...body, service_id: service }),
    });
  }
  function operation(current: Setup) {
    return {
      setup_id: current.id,
      expected_revision: current.revision,
      idempotency_key: requestID(),
    };
  }
  async function run(action: () => Promise<void>) {
    setPending(true);
    setError(null);
    setNotice("");
    try {
      await action();
      await cache.invalidateQueries({ queryKey: [service] });
    } catch (cause) {
      setError(cause instanceof Error ? cause : new Error("Setup failed"));
      await progress.refetch();
    } finally {
      setPending(false);
    }
  }
  async function save(): Promise<Setup> {
    const input = {
      name,
      ...(provider === "aws" ? { account_id: account, role_arn: role } : {}),
    };
    let current = setup;
    if (!current) {
      current = await post<Setup>("begin", {
        provider,
        input,
        idempotency_key: requestID(),
      });
      setParams({ page: "Connections", provider, setup: current.id });
    }
    current = await post<Setup>("update", { ...operation(current), input });
    cache.setQueryData([service, "setup", current.id], current);
    return current;
  }
  async function connect() {
    const current = await save();
    const handoff = await post<Schema["AuthorizationHandoff"]>(
      "authorize",
      operation(current),
    );
    const url = new URL(handoff.url);
    if (url.origin !== window.location.origin)
      throw new Error(
        "The configured callback origin differs from this application. Ask the operator to check KSI_PUBLIC_URL.",
      );
    window.location.assign(url.href);
  }
  function download() {
    if (!material) return;
    downloadText("fed-aws-setup.sh", material.cli, "text/x-shellscript");
  }
  function downloadText(
    filename: string,
    text: string,
    type = "application/json",
  ) {
    const blob = new Blob([text], { type });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
  }
  const callbackError = setup?.error_code || params.get("setup_error");
  const recovery: Record<string, string> = {
    administrator_approval_pending:
      "Ask your organization administrator to approve the App, then retry Connect.",
    workspace_policy_restricted:
      "Ask your Workspace administrator to approve the Fed OAuth application under API access controls; ask the Fed operator to verify the consent audience and Admin SDK enablement.",
    customer_identity_permission_required:
      "Consent must include read-only Directory customer access so Fed can verify the Workspace customer. Retry as an authorized administrator.",
    offline_consent_required:
      "Retry Google consent and approve offline access so collection can continue after this browser session.",
    wrong_tenant:
      "Choose the account already bound to this connection. Use a new connection for a different account.",
    role_trust_denied:
      "Check the Fed principal, generated external ID and customer trust policy. Allow IAM propagation, then verify again.",
    installation_suspended:
      "Ask the GitHub organization administrator to restore the suspended App installation, then retry Connect.",
    installation_removed:
      "Reinstall the Fed GitHub App for the intended repositories, then retry Connect.",
  };
  return (
    <section
      className="panel connection-form"
      aria-label={`${human(provider)} guided setup`}
    >
      <p className="eyebrow">Guided connection</p>
      <h2>
        Connect{" "}
        {provider === "google_workspace"
          ? "Google Workspace"
          : provider === "github"
            ? "GitHub"
            : "AWS"}
      </h2>
      <ol className="setup-progress" aria-label="Setup progress">
        <li aria-current={!setup ? "step" : undefined}>Prerequisites</li>
        <li
          aria-current={
            setup && !["confirm", "complete"].includes(setup.step)
              ? "step"
              : undefined
          }
        >
          Grant read access
        </li>
        <li aria-current={setup?.step === "confirm" ? "step" : undefined}>
          Confirm identity
        </li>
        <li aria-current={setup?.step === "complete" ? "step" : undefined}>
          Discover and select sources
        </li>
      </ol>
      <p>
        Connect verifies access. Discovery, evidence collection and assessment
        review are separate steps.
      </p>
      {readiness.isPending && <output>Checking deployment readiness…</output>}
      {readiness.error && (
        <ErrorPanel
          error={readiness.error}
          retry={() => {
            void readiness.refetch();
          }}
        />
      )}
      {progress.error && (
        <ErrorPanel
          error={progress.error}
          retry={() => {
            void progress.refetch();
          }}
        />
      )}
      {readiness.data && (
        <>
          <h3>Prerequisites</h3>
          <ul>
            {readiness.data.customer_actions?.map((text) => (
              <li key={text}>{text}</li>
            ))}
          </ul>
          {!readiness.data.ready && (
            <div aria-live="polite">
              <h3>Operator setup required</h3>
              <ul>
                {readiness.data.operator_actions?.map((text) => (
                  <li key={text}>{text}</li>
                ))}
              </ul>
              <p>{readiness.data.missing_configuration?.join(", ")}</p>
              <p>{human(readiness.data.error_code || "")}</p>
              <button
                type="button"
                onClick={() => {
                  void readiness.refetch();
                }}
              >
                Check readiness again
              </button>
            </div>
          )}
          <details>
            <summary>Read permissions and application registration</summary>
            <ul>
              {readiness.data.permissions?.map((text) => (
                <li key={text}>
                  <code>{text}</code>
                </li>
              ))}
            </ul>
            <ul>
              {readiness.data.operator_actions?.map((text) => (
                <li key={text}>{text}</li>
              ))}
            </ul>
          </details>
        </>
      )}
      {callbackError && (
        <p role="alert">
          {human(callbackError)}.{" "}
          {recovery[callbackError] ||
            "Review the selected account and provider permissions, then retry Connect. Your setup is saved."}
        </p>
      )}
      {error && <ErrorPanel error={error} retry={() => setError(null)} />}
      <output aria-live="polite">{pending ? "Saving setup…" : notice}</output>
      {terminal ? (
        <>
          <p>Setup {human(setup.step)}.</p>
          {setup.step === "complete" && (
            <button
              type="button"
              onClick={() =>
                setParams({
                  page: "Connections",
                  connection: setup.connection_id || "",
                })
              }
            >
              Discover sources
            </button>
          )}
          <button
            type="button"
            onClick={() => {
              setParams({ page: "Connections", provider });
              setMaterial(null);
              loaded.current = null;
            }}
          >
            Start another setup
          </button>
        </>
      ) : (
        <fieldset disabled={pending || !canConnect}>
          <label>
            Connection name
            <input
              value={name}
              readOnly={setup?.step === "confirm"}
              onChange={(e) => setName(e.target.value)}
              maxLength={200}
              required
              autoComplete="off"
            />
          </label>
          {provider === "aws" && (
            <>
              <p>
                Fed assumes a role you explicitly trust. All inspection is
                read-only in us-east-1.
              </p>
              <label>
                Customer AWS account ID
                <input
                  value={account}
                  readOnly={setup?.step === "confirm"}
                  onChange={(e) => {
                    setAccount(e.target.value);
                    setMaterial(null);
                  }}
                  inputMode="numeric"
                  pattern="[0-9]{12}"
                  maxLength={12}
                  required
                />
              </label>
              <label>
                Setup method
                <select
                  value={method}
                  onChange={(e) => setMethod(e.target.value)}
                >
                  <option value="cli">AWS CLI</option>
                  <option value="manual">Manual IAM setup</option>
                </select>
              </label>
              <button
                type="button"
                disabled={
                  !readiness.data?.ready || !name || account.length !== 12
                }
                onClick={() =>
                  void run(async () => {
                    const current = await save();
                    const generated = await post<Material>(
                      "aws-material",
                      operation(current),
                    );
                    setMaterial(generated);
                    if (!role) setRole(generated.role_arn);
                  })
                }
              >
                Generate role setup
              </button>
              {material && (
                <section aria-label="Generated AWS role setup">
                  <p>
                    Suggested role: <code>{material.role_arn}</code>
                  </p>
                  {method === "cli" ? (
                    <>
                      <pre className="setup-code">{material.cli}</pre>
                      <button
                        type="button"
                        onClick={() =>
                          void run(async () => {
                            await navigator.clipboard.writeText(material.cli);
                            setNotice("CLI script copied.");
                          })
                        }
                      >
                        Copy CLI script
                      </button>
                      <button type="button" onClick={download}>
                        Download script
                      </button>
                    </>
                  ) : (
                    <>
                      <ol>
                        {material.instructions?.map((text) => (
                          <li key={text}>{text}</li>
                        ))}
                      </ol>
                      <h4>Trust policy</h4>
                      <button
                        type="button"
                        onClick={() =>
                          void run(async () => {
                            await navigator.clipboard.writeText(
                              JSON.stringify(material.trust_policy, null, 2),
                            );
                            setNotice("Trust policy copied.");
                          })
                        }
                      >
                        Copy trust policy
                      </button>
                      <button
                        type="button"
                        onClick={() =>
                          downloadText(
                            "fed-trust-policy.json",
                            JSON.stringify(material.trust_policy, null, 2),
                          )
                        }
                      >
                        Download trust policy
                      </button>
                      <pre className="setup-code">
                        {JSON.stringify(material.trust_policy, null, 2)}
                      </pre>
                      <h4>Read policy</h4>
                      <button
                        type="button"
                        onClick={() =>
                          void run(async () => {
                            await navigator.clipboard.writeText(
                              JSON.stringify(material.read_policy, null, 2),
                            );
                            setNotice("Read policy copied.");
                          })
                        }
                      >
                        Copy read policy
                      </button>
                      <button
                        type="button"
                        onClick={() =>
                          downloadText(
                            "fed-read-policy.json",
                            JSON.stringify(material.read_policy, null, 2),
                          )
                        }
                      >
                        Download read policy
                      </button>
                      <pre className="setup-code">
                        {JSON.stringify(material.read_policy, null, 2)}
                      </pre>
                    </>
                  )}
                </section>
              )}
              <label>
                Customer role ARN
                <input
                  value={role}
                  readOnly={setup?.step === "confirm"}
                  onChange={(e) => setRole(e.target.value)}
                  autoComplete="off"
                />
              </label>
              {setup && (
                <>
                  <p>
                    Replacing the external ID invalidates existing delegated
                    access until the customer trust policy is updated and
                    verified.
                  </p>
                  <button
                    type="button"
                    onClick={() =>
                      void run(async () => {
                        await post<Setup>("aws-rotate", operation(setup));
                        setMaterial(null);
                        setNotice(
                          "External ID replaced. Generate the new trust policy, update the customer role, then verify access.",
                        );
                      })
                    }
                  >
                    Replace external ID
                  </button>
                </>
              )}
              <button
                type="button"
                disabled={!readiness.data?.ready || !role || !name}
                onClick={() =>
                  void run(async () => {
                    const current = await save();
                    await post<Setup>("aws-verify", operation(current));
                    setMaterial(null);
                    setNotice(
                      "Role access verified. Confirm the account below.",
                    );
                  })
                }
              >
                Verify role access
              </button>
            </>
          )}
          {provider !== "aws" && setup?.step !== "confirm" && (
            <button
              type="button"
              disabled={!name || !readiness.data?.ready}
              onClick={() => void run(connect)}
            >
              {provider === "github"
                ? "Connect with GitHub App"
                : "Connect with Google"}
            </button>
          )}
          {setup?.step === "confirm" && (
            <section>
              <h3 ref={confirmation} tabIndex={-1}>
                Confirm verified identity
              </h3>
              <p>
                Provider account: <strong>{setup.verified_account_id}</strong>
              </p>
              <button
                type="button"
                onClick={() =>
                  void run(async () => {
                    const complete = await post<Setup>(
                      "complete",
                      operation(setup),
                    );
                    setMaterial(null);
                    setParams({
                      page: "Connections",
                      connection: complete.connection_id || "",
                    });
                  })
                }
              >
                Confirm connection
              </button>
              <button
                type="button"
                onClick={() =>
                  void run(async () => {
                    await save();
                    setNotice(
                      "Returned to setup. Verify access again before confirming.",
                    );
                  })
                }
              >
                Back to setup
              </button>
            </section>
          )}
          {setup && (
            <button
              type="button"
              onClick={() =>
                void run(async () => {
                  await post<Setup>("cancel", operation(setup));
                  setMaterial(null);
                })
              }
            >
              Cancel setup
            </button>
          )}
        </fieldset>
      )}
    </section>
  );
}
