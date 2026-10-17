import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useId, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  APIError,
  type Form,
  human,
  type Page,
  request,
  type Value,
} from "./api";

const examples: Record<string, Value> = {
  accounts: { id: "", regions: ["us-east-1"], clusters: [] },
  repositories: { name: "", commit: "", paths: [] },
  evidence: { id: "", revision: 1 },
  measures: { id: "", revision: 1 },
  overrides: { requirement_id: "", freshness_hours: 48 },
};
function initialFields(form: Form) {
  return Object.fromEntries(
    form.Fields.map((f) => {
      let value = f.Value;
      if (f.Name === "payload" && form.Action.endsWith("evidence-source")) {
        try {
          value = JSON.stringify({
            account_id: "",
            region: "",
            cluster: "",
            repository: "",
            commit: "",
            access_hint: "",
            paths: [],
            requirement_ids: [],
            ...JSON.parse(value),
          });
        } catch {}
      }
      return [f.Name, value];
    }),
  );
}
export function ErrorPanel({
  error,
  retry,
}: {
  error: Error;
  retry?: () => void;
}) {
  const e =
    error instanceof APIError
      ? error
      : new APIError(
          "unknown",
          "Unable to complete this action",
          error.message,
        );
  return (
    <div className="panel error-panel" role="alert">
      <div>
        <h2>{e.title}</h2>
        <p>{e.explanation}</p>
        {e.reference && (
          <p className="small muted">Diagnostic reference: {e.reference}</p>
        )}
        {e.status === 401 ? (
          <a className="button" href="/auth/login">
            Sign in
          </a>
        ) : (
          retry && (
            <button type="button" onClick={retry}>
              Try again
            </button>
          )
        )}
      </div>
    </div>
  );
}

export function StructuredInput({
  name,
  value,
  onChange,
  errors = {},
}: {
  name: string;
  value: Value;
  onChange: (value: Value) => void;
  errors?: Record<string, string>;
}) {
  const controlID = useId();
  if (name === "overrides") {
    const policies =
      value && typeof value === "object" && !Array.isArray(value) ? value : {};
    return (
      <fieldset className="structured">
        <legend>Collection policy</legend>
        <p className="muted">Enable only policies approved for this service.</p>
        {[
          "minimum_retention_days",
          "approved_cidrs",
          "namespace_exclusions",
        ].map((key) => (
          <div key={key}>
            <label className="check-label">
              <input
                type="checkbox"
                checked={key in policies}
                onChange={(e) => {
                  const next = { ...policies };
                  if (e.target.checked)
                    next[key] = key === "minimum_retention_days" ? 7 : [];
                  else delete next[key];
                  onChange(next);
                }}
              />
              {human(key)}
            </label>
            {key in policies && (
              <StructuredInput
                name={key}
                value={policies[key]}
                errors={errors}
                onChange={(next) => onChange({ ...policies, [key]: next })}
              />
            )}
          </div>
        ))}
      </fieldset>
    );
  }
  if (
    value === null &&
    [
      "accounts",
      "repositories",
      "paths",
      "regions",
      "clusters",
      "requirement_ids",
      "evidence",
      "measures",
      "expected",
    ].includes(name)
  )
    value = [];
  if (Array.isArray(value))
    return (
      <fieldset className="structured">
        <legend>{human(name)}</legend>
        {errors[name] && (
          <p className="field-error" role="alert">
            {errors[name]}
          </p>
        )}
        {value.map((item, index) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: controlled editor rows have no component-local value state
          <div className="array-item" key={`${name}-${index}`}>
            <StructuredInput
              name={`${name} ${index + 1}`}
              value={item}
              errors={errors}
              onChange={(next) =>
                onChange(value.map((v, i) => (i === index ? next : v)))
              }
            />
            <button
              type="button"
              onClick={() => onChange(value.filter((_, i) => i !== index))}
            >
              Remove {human(name)} {index + 1}
            </button>
          </div>
        ))}
        <button
          type="button"
          onClick={() =>
            onChange([...value, structuredClone(examples[name] ?? "")])
          }
        >
          Add {human(name)}
        </button>
      </fieldset>
    );
  if (value !== null && typeof value === "object")
    return (
      <fieldset className="structured">
        <legend>{human(name)}</legend>
        {Object.entries(value).map(([key, item]) => (
          <StructuredInput
            key={key}
            name={key}
            value={item}
            errors={errors}
            onChange={(next) => onChange({ ...value, [key]: next })}
          />
        ))}
      </fieldset>
    );
  if (name === "type")
    return (
      <label className="input-label">
        Source type
        <select
          value={String(value || "document")}
          onChange={(e) => onChange(e.target.value)}
        >
          {[
            "document",
            "repository",
            "aws",
            "kubernetes",
            "operational_records",
          ].map((v) => (
            <option key={v} value={v}>
              {human(v)}
            </option>
          ))}
        </select>
      </label>
    );
  if (typeof value === "boolean")
    return (
      <label className="check-label">
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
        />
        {human(name)}
      </label>
    );
  return (
    <label className="input-label">
      {human(name)}
      <input
        aria-invalid={!!errors[name]}
        aria-label={human(name)}
        aria-describedby={errors[name] ? `${controlID}-error` : undefined}
        type={typeof value === "number" ? "number" : "text"}
        min={
          typeof value === "number"
            ? name.includes("hours")
              ? 1
              : 0
            : undefined
        }
        readOnly={
          name === "scope_revision" ||
          name === "revision" ||
          name === "next_run_at"
        }
        max={
          name === "minimum_retention_days"
            ? 35
            : name.includes("hours")
              ? 8760
              : undefined
        }
        pattern={
          name === "commit"
            ? "[a-f0-9]{40}"
            : name === "account_id"
              ? "[0-9]{12}"
              : undefined
        }
        title={
          name === "commit"
            ? "Use the full 40-character commit hash"
            : name === "account_id"
              ? "Use a 12-digit AWS account ID"
              : undefined
        }
        value={value ?? ""}
        onChange={(e) =>
          onChange(
            typeof value === "number" ? Number(e.target.value) : e.target.value,
          )
        }
      />
      {errors[name] && (
        <span id={`${controlID}-error`} className="field-error">
          {errors[name]}
        </span>
      )}
    </label>
  );
}

export function ActionForm({
  form,
  csrf,
  allowed = true,
}: {
  form: Form;
  csrf: string;
  allowed?: boolean;
}) {
  const formID = useId();
  const query = useQueryClient();
  const navigate = useNavigate();
  const [values, setValues] = useState<Record<string, string>>(() =>
    initialFields(form),
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<Error>();
  const [result, setResult] = useState<Page>();
  const [saved, setSaved] = useState(false);
  useEffect(() => {
    if (saved) setValues(initialFields(form));
  }, [saved, form]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (pending) return;
    setPending(true);
    setError(undefined);
    setSaved(false);
    try {
      const action = form.Action.startsWith("/services")
        ? `/api/ui/v1${form.Action}`
        : form.Action;
      const out = await request<Page & { redirect?: string }>(action, {
        method: "POST",
        headers: {
          "X-CSRF-Token": csrf,
          "Content-Type": "application/x-www-form-urlencoded",
        },
        body: new URLSearchParams(values),
      });
      setSaved(true);
      await query.invalidateQueries();
      if (out.redirect) {
        navigate(out.redirect);
      } else {
        setResult(out);
      }
    } catch (e) {
      setError(e as Error);
      if (e instanceof APIError && e.code === "revision_conflict")
        await query.invalidateQueries();
    } finally {
      setPending(false);
    }
  }
  return (
    <section className="panel">
      <form onSubmit={submit}>
        <h2>{form.Title.replace("Source · ", "Edit source · ")}</h2>
        {form.Fields.map((f) => {
          if (f.Type === "hidden") return null;
          if (
            (f.Name === "evidence" || f.Name === "measures") &&
            form.References
          ) {
            const selected =
              JSON.parse(values[f.Name] || "[]") ||
              ([] as { id: string; revision: number }[]);
            const options = [
              ...new Map(
                form.References.filter((o) => o.Kind === f.Name).map((o) => [
                  `${o.ID}:${o.Revision}`,
                  o,
                ]),
              ).values(),
            ];
            return (
              <fieldset className="structured" key={f.Name}>
                <legend>
                  {f.Name === "evidence"
                    ? "Supporting evidence"
                    : "Supporting measures"}
                </legend>
                {options.map((o) => (
                  <label className="check-label" key={`${o.ID}:${o.Revision}`}>
                    <input
                      type="checkbox"
                      checked={selected.some(
                        (s: { id: string; revision: number }) =>
                          s.id === o.ID && s.revision === o.Revision,
                      )}
                      onChange={(e) => {
                        const next = selected.filter(
                          (s: { id: string; revision: number }) =>
                            s.id !== o.ID || s.revision !== o.Revision,
                        );
                        if (e.target.checked)
                          next.push({ id: o.ID, revision: o.Revision });
                        setValues({
                          ...values,
                          [f.Name]: JSON.stringify(next),
                        });
                      }}
                    />
                    {o.Label || "Supporting record"} · revision {o.Revision}
                  </label>
                ))}
                <p className="small muted">
                  Evidence choices include this page and cited artifacts.
                  Existing selections are retained when browsing other pages.
                </p>
              </fieldset>
            );
          }
          let structured: Value | undefined;
          if (
            f.Name === "payload" ||
            f.Name === "evidence" ||
            f.Name === "measures"
          ) {
            try {
              structured = JSON.parse(values[f.Name]);
            } catch {
              structured = undefined;
            }
          }
          if (structured !== undefined)
            return (
              <StructuredInput
                key={f.Name}
                name={f.Name === "payload" ? "Configuration" : f.Name}
                value={structured}
                errors={error instanceof APIError ? error.fields : {}}
                onChange={(v) =>
                  setValues({ ...values, [f.Name]: JSON.stringify(v) })
                }
              />
            );
          if (f.Type === "checkbox")
            return (
              <label className="check-label" key={f.Name}>
                <input
                  type="checkbox"
                  checked={values[f.Name] === "yes"}
                  onChange={(e) =>
                    setValues({
                      ...values,
                      [f.Name]: e.target.checked ? "yes" : "",
                    })
                  }
                />
                {f.Label}
              </label>
            );
          const label = f.Label.replace(/ JSON.*$/, "");
          return (
            <label
              className="input-label"
              key={f.Name}
              htmlFor={`${formID}-${f.Name}`}
            >
              {label}
              {f.Type === "select" ? (
                <select
                  id={`${formID}-${f.Name}`}
                  value={values[f.Name]}
                  onChange={(e) =>
                    setValues({ ...values, [f.Name]: e.target.value })
                  }
                >
                  {f.Options?.map((o) => (
                    <option key={o} value={o}>
                      {o || "None"}
                    </option>
                  ))}
                </select>
              ) : f.Type === "textarea" ? (
                <textarea
                  id={`${formID}-${f.Name}`}
                  value={values[f.Name]}
                  onChange={(e) =>
                    setValues({ ...values, [f.Name]: e.target.value })
                  }
                />
              ) : (
                <input
                  id={`${formID}-${f.Name}`}
                  value={values[f.Name]}
                  onChange={(e) =>
                    setValues({ ...values, [f.Name]: e.target.value })
                  }
                />
              )}
            </label>
          );
        })}
        {error && <ErrorPanel error={error} />}{" "}
        {error instanceof APIError && error.code === "revision_conflict" && (
          <section>
            <h3>Latest saved version</h3>
            {form.Fields.filter((f) => f.Type !== "hidden").map((f) => (
              <div key={f.Name}>
                <strong>{f.Label}</strong>
                <Readable value={parseValue(f.Value)} />
              </div>
            ))}
            <button
              type="button"
              onClick={() => {
                setValues((current) => ({
                  ...current,
                  ...Object.fromEntries(
                    form.Fields.filter(
                      (f) => f.Type === "hidden" && f.Name !== "key",
                    ).map((f) => [f.Name, f.Value]),
                  ),
                }));
                setError(undefined);
              }}
            >
              I compared the changes — keep my entries against this revision
            </button>
          </section>
        )}
        <button className="button" type="submit" disabled={pending || !allowed}>
          {pending ? "Saving…" : form.Button}
        </button>
        {!allowed && (
          <p className="muted">
            Ask a service administrator for access to this action.
          </p>
        )}
        {saved && <output className="saved">Saved successfully.</output>}
      </form>
      {result && (
        <>
          <p className="muted">{result.Notice}</p>
          {result.Sections?.map((s) => (
            <section key={s.Title}>
              <h3>{s.Title}</h3>
              <Readable value={parseValue(s.Text)} />
            </section>
          ))}
          {result.Forms?.map((f) => (
            <ActionForm key={f.Action} form={f} csrf={csrf} />
          ))}
        </>
      )}
    </section>
  );
}
export function parseValue(raw: string): Value {
  try {
    return JSON.parse(raw);
  } catch {
    return raw;
  }
}
export function Readable({ value }: { value: Value }) {
  if (value === null) return <span className="muted">Not recorded</span>;
  if (Array.isArray(value))
    return value.length ? (
      <ul className="readable-list">
        {value.map((v, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: immutable read-only record items
          <li key={`item-${i}`}>
            <Readable value={v} />
          </li>
        ))}
      </ul>
    ) : (
      <p className="muted">None recorded</p>
    );
  if (typeof value === "object")
    return (
      <dl className="readable">
        {Object.entries(value).map(([key, v]) => (
          <div key={key}>
            <dt>{human(key)}</dt>
            <dd>
              <Readable value={v} />
            </dd>
          </div>
        ))}
      </dl>
    );
  return (
    <span className="prose">
      {typeof value === "boolean" ? (value ? "Yes" : "No") : value}
    </span>
  );
}
