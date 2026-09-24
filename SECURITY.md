# Security Policy

Rainy is a self-hosted music server with library management. This policy
describes which versions receive security fixes and how to report a
vulnerability privately.

Deployment hardening guidance is in
[docs/operations/security.md](docs/operations/security.md). For a factual
description of stored data, see [PRIVACY.md](PRIVACY.md). The
[security documentation map](docs/security/index.md) explains which document
to use for reporting, deployment, development, and privacy questions.

## Supported Versions

Security fixes target the latest tagged release.

| Version | Support |
| --- | --- |
| Latest release matching [VERSION](VERSION) | Supported |
| `main` after the latest release | Reports accepted; development code |
| Older releases and unofficial builds | Not supported |

Please reproduce an issue against the latest release or current `main` when
practical.

## Security Model

Rainy has these intended trust boundaries:

- Library data, media, and every API except the health check, sign-in, and
  first-run setup require authentication. First-run setup is intended to be
  available only while no user exists.
- Listeners may read the library and change only their own state. Managers may
  change music files through management features. Administrators manage users,
  libraries, and server settings. Crossing one of these boundaries without the
  permission is in scope.
- The configured library roots and the data directory are trusted operator
  mounts. Reading, writing, moving, or deleting files outside them through the
  API, tags, uploads, or file names is in scope.
- Passwords are stored reversibly encrypted with `secret.key` because the
  Subsonic token protocol requires it. Disclosure of passwords, session
  tokens, API keys, or `secret.key` through Rainy is in scope.

## Reporting a Vulnerability

Do not open a public issue, discussion, or pull request for an undisclosed
vulnerability.

Use [GitHub Private Vulnerability Reporting](https://github.com/yexca/rainy/security/advisories/new).

Include:

- The affected version, commit, or image digest.
- The deployment topology (direct port, reverse proxy, VPN) and relevant
  configuration such as `RAINY_TRUST_PROXY`.
- The required account role or other prerequisites.
- Minimal reproduction steps using synthetic data.
- The expected and actual behavior.
- The security impact and any suggested mitigation.

Do not attach real databases, `secret.key`, passwords, session cookies, API
keys, personal paths, or music files. Redact logs before submitting them. Use
reserved domains, documentation IP ranges, and synthetic file names whenever
possible. Do not test against instances you do not own or have permission to
test.

## Examples of In-Scope Reports

- Authentication, session, API-key, or Subsonic authentication bypasses.
- A listener performing manager or administrator actions, or reading another
  user's private playlists or state.
- Creating an account through first-run setup on an initialized instance.
- Path traversal or symlink escapes that read, write, move, or delete files
  outside the library roots or data directory.
- Cross-site scripting through tags, lyrics, file names, or playlist names.
- Cross-site request forgery against state-changing endpoints.
- Command injection through ffmpeg arguments or file names.
- Disclosure of credentials, `secret.key`, or host paths to unauthorized users.
- Compromise of official release images or the release process.

## Generally Out of Scope

- Traffic interception when an operator deliberately exposes Rainy over plain
  HTTP to an untrusted network.
- Subsonic credentials in request URLs, which the Subsonic protocol requires;
  use HTTPS or API keys.
- Issues that require prior control of the host, the database, `secret.key`,
  or the mounted folders.
- Enabling `RAINY_TRUST_PROXY` while clients can reach Rainy directly.
- The limitations documented in
  [Deployment security](docs/operations/security.md#known-limitations).
- Denial-of-service testing, social engineering, physical attacks, or automated
  scanner output without a demonstrated impact.

## Response and Disclosure

We aim to acknowledge a report within five business days and to provide an
initial assessment within ten business days. Remediation time depends on
severity and complexity.

Please allow coordinated remediation before public disclosure. When
appropriate, the fix, release notes, and a GitHub security advisory are
published together. Reporter credit is offered unless you prefer to stay
anonymous.

## Safe Harbor

Good-faith research that follows this policy is welcome, and the maintainers
will not pursue legal action against it. Please:

- Use accounts, instances, and data that you own or are authorized to test.
- Stop after obtaining the minimum evidence needed.
- Avoid privacy violations, data destruction, and service degradation.
- Keep the issue private during coordinated remediation.

Rainy does not operate a paid bug-bounty program.
