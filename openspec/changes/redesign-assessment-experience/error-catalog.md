# Operator recovery catalog

`internal/observation/recovery.go` owns collector recovery descriptions. Stored
observations retain their original machine code; new collector observations also
carry additive recovery context. Historical observations use the same catalog.

| Cause | Operator action | Environment |
| --- | --- | --- |
| Typed AWS expired token | Refresh configured AWS sign-in, retry collection | Server collector |
| Typed AWS access denied | Grant target read access, retry | Server collector |
| Kubernetes namespace forbidden | Allow namespace listing on the named cluster | Configured Kubernetes context |
| Missing Kubernetes binding | Configure context mapping or agent access hint | Server or external agent |
| Missing GitHub credential | Configure repository read access | Server collector or external agent |
| Repository file unavailable | Check pinned commit, path and read access | Reported source |
| Generic credential/account check failure | Verify profile, sign-in and account binding | Collector environment |
| Unknown legacy collection code | Inspect source access and exact diagnostics | Recorded source kind |
| Browser validation | Correct named field; retain other inputs | Browser form |
| Revision conflict | Compare latest draft/configuration; explicitly retry | Browser form |
| Expired session / CSRF | Sign in or refresh session | Browser |
| Forbidden action | Request service permission | Browser |
| Unknown backend error | Retry/read diagnostics using logged reference | Service |

Unknown errors do not receive guessed credential-expiry labels. Diagnostic
references are logged server-side; SQL and raw exceptions are excluded from API
error bodies. Source editing does not claim to repair external credentials.
Requirement links, source settings, collection forms and scoped agent prompts
are the recovery destinations; there is no acknowledgement-as-resolution action.

Legacy limitations: historical flat error strings do not identify an AWS profile
or Kubernetes verb beyond the recorded target/code. Such records retain their
known context and a generic investigation instruction. Fresh collection preserves
typed AWS and Kubernetes authorization failures where available.
