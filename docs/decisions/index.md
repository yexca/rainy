# Architecture Decision Records

ADRs record durable design decisions and the reasons behind them.

- [ADR-0001: Pure Go runtime](ADR-0001-pure-go-runtime.md)
- [ADR-0002: Simulated folder browsing](ADR-0002-simulated-folder-browsing.md)
- [ADR-0003: SQLite first](ADR-0003-sqlite-first.md)

## When To Add An ADR

Add an ADR when a decision changes long-term architecture, persistence,
deployment, client compatibility, or developer workflow. Routine
implementation details belong in code comments, the
[architecture contract](../architecture/contract.md), or the area docs.

Number ADRs sequentially and never reuse a number. To reverse a decision, add
a new ADR that supersedes the old one and update the old one's status.

## Related Docs

- [Architecture](../architecture/index.md)
- [Development](../development/local-dev.md)
