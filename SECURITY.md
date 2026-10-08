# Security policy

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting feature for this repository. Do not open a public issue containing a credential, private configuration file, exploit details, or an unredacted HarnessScope report.

Include the affected version or commit, operating system, minimal reproduction using synthetic data, and the expected versus observed behavior. Remove real tokens, internal hostnames, and user paths before submission.

## Scope

Security-sensitive areas include secret redaction, report escaping, known-path discovery boundaries, fix source-hash checks, backup permissions, atomic replacement, rollback isolation, loopback authentication, origin/host checks, revision serialization, snapshot storage and release artifact integrity.

HarnessScope is a local diagnostic tool and cannot guarantee detection of every proprietary credential format. Users should review generated reports before publishing them.

## v0.2 local dashboard

`serve` binds only `127.0.0.1`. There is no remote mode, cloud storage, telemetry, remote config fetching or automatic upload. Configured MCP servers and hooks are not executed; detected client executables may be invoked for version reporting. Treat client executables and local files as part of your machine's trust boundary.

The startup URL contains a random session token in its fragment. Keep it private: anyone who can read it and reach your loopback listener can use that session. It is consumed into browser memory, removed from the address bar and never persisted in cookies, localStorage or sessionStorage. Reloading requires the launch URL. Stop the process to end the session. Static assets are public on loopback, but all API routes reject missing or invalid tokens. Exact Host/Origin checks, strict JSON decoding, bounded request bodies, CSP and no-store responses provide additional defenses.

Mutations require the current revision. Browser apply accepts only selected SAFE plans after a confirmation preview; REVIEW/BLOCKED cannot be auto-applied. Source hashes are checked before writes, operations are serialized, backups precede writes, and failed verification rolls back. A stale request is never silently replayed. These defenses do not protect against another process already authorized to read your files or launch URL.

## Sharing and storage

Snapshots are sanitized, atomic local files with user-only permissions. Diagnostic ZIPs contain sanitized JSON/HTML, optional normalized drift, a README and a hashed manifest; raw configs, backups and tokens are excluded. SHA-256 detects corruption against a trusted checksum list; it does not independently authenticate a publisher.

Redaction and root/home collapsing reduce exposure but cannot classify every private identifier or arbitrary path spelling. Baseline save/list and output-location messages intentionally show local destination paths; do not publish terminal transcripts without review. Inspect every report, snapshot and ZIP member before uploading publicly. Drift omits raw before/after values and cannot establish semantic equivalence.

The committed public screenshot uses only synthetic fixture data. The reviewed Linux visual gate is currently open and blocks the release workflow; local functional test success is not a published security audit or a claim that this gate passed.
