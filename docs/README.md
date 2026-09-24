# Rainy Documentation

This is the public documentation for Rainy. It is organized by reader task:
using the product, running it, changing it, and reviewing design decisions.

## Start Here

- [User guide](user/index.md): library layout, playback, management, and
  Subsonic clients.
- [Overview](overview.md)
- [Getting started](user/getting-started.md)
- [Configuration](operations/configuration.md)
- [Troubleshooting](operations/troubleshooting.md)

## By Area

- [Architecture](architecture/index.md): system boundaries, packages, data,
  and the two HTTP APIs. The precise contract lives in
  [architecture/contract.md](architecture/contract.md).
- [User guide](user/index.md): user-visible screens and behavior.
- [Operations](operations/configuration.md): configuration, Docker and NAS
  deployment, reverse proxies, the database, security, and troubleshooting.
- [Development](development/local-dev.md): local setup, testing, design,
  migrations, and the commit and release workflow.
- [Security documentation](security/index.md): reporting, deployment,
  development, and privacy guidance.
- [Decisions](decisions/index.md): durable architecture decision records.
- [History](history/index.md): release notes.

## Reading Paths

New users should read:

- [Overview](overview.md)
- [Getting started](user/getting-started.md)
- [Library layout and scanning](user/library.md)
- [Subsonic clients](user/clients.md)

Operators should read:

- [Docker and NAS guides](operations/docker.md)
- [Configuration](operations/configuration.md)
- [Reverse proxy and HTTPS](operations/reverse-proxy.md)
- [Database and backups](operations/database.md)
- [Deployment security](operations/security.md)
- [Privacy and data handling](../PRIVACY.md)

Developers should read:

- [Repository agent guide](../AGENTS.md)
- [Architecture contract](architecture/contract.md)
- [Backend](architecture/backend.md) and [Frontend](architecture/frontend.md)
- [Design system](development/design.md)
- [Testing](development/testing.md)
- [Secure development](development/security.md)
- [Commit and release](development/commit-and-release.md)

Security reporters should read:

- [Security policy](../SECURITY.md)
- [Security documentation map](security/index.md)

## Documentation Rules

- The [architecture contract](architecture/contract.md) is the precise source
  of truth for schema, package APIs, endpoints, TypeScript types, and the
  design system. Update it in the same change as the code it describes. The
  other architecture pages summarize and link to its sections instead of
  copying its tables.
- User-visible behavior belongs in the [User guide](user/index.md).
- System boundaries belong in [Architecture](architecture/index.md).
- Runtime instructions belong in [Operations](operations/configuration.md).
- Local developer workflow belongs in [Development](development/local-dev.md).
- Security implementation rules belong in
  [Secure development](development/security.md).
- Durable design choices belong in [ADRs](decisions/index.md).
- Release notes belong in [History](history/index.md) and are never the only
  description of current behavior.
- English is canonical. The translated READMEs under `readme/` summarize the
  English README and link back to the English documentation.
- Use reserved examples in every public document: `example.com` domains,
  `192.0.2.0/24` addresses, `/path/to/music`, and obviously fake credentials.
