# ADR-0003: SQLite First

## Status

Accepted.

## Context

Rainy is a personal music server that runs as a single container on modest NAS
hardware. Operators expect to back up and restore it by copying a folder, and
should not have to run or tune a separate database server.

## Decision

Use one SQLite database file in the data directory as the only durable store,
opened with WAL mode, a read-only reader pool, and a single writer connection.
Schema changes are numbered, embedded SQL migrations applied at startup.

## Consequences

- Installation needs no database service, and backups are file copies of the
  data folder while Rainy is stopped.
- Only one Rainy process may use a data directory. Horizontal scaling is out of
  scope.
- All writes are serialized through one connection, so long write transactions
  delay other writers. Keep transactions short and batch scanner writes.
- The data folder should live on a local disk; SQLite over network file systems
  risks locking problems.
