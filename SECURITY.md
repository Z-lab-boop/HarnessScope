# Security policy

## Reporting a vulnerability

Please use GitHub's private vulnerability reporting feature for this repository. Do not open a public issue containing a credential, private configuration file, exploit details, or an unredacted HarnessScope report.

Include the affected version or commit, operating system, minimal reproduction using synthetic data, and the expected versus observed behavior. Remove real tokens, internal hostnames, and user paths before submission.

## Scope

Security-sensitive areas include secret redaction, report escaping, known-path discovery boundaries, fix source-hash checks, backup permissions, atomic replacement, rollback isolation, and release artifact integrity.

HarnessScope is a local diagnostic tool and cannot guarantee detection of every proprietary credential format. Users should review generated reports before publishing them.
