# Operations

Rainy runs as one Docker container with a persistent data directory and one
or more music libraries. Start with [Docker](docker.md) for NAS deployment.

| Task | Guide |
| --- | --- |
| Install, configure mounts, and set permissions | [Docker](docker.md) |
| Configure environment and administrator settings | [Configuration](configuration.md) |
| Expose Rainy through HTTPS | [Reverse proxy](reverse-proxy.md) |
| Back up, restore, and maintain state | [Database](database.md) |
| Understand outages and recovery limits | [Reliability](reliability.md) |
| Harden deployment | [Deployment security](security.md) |
| Diagnose a failure | [Troubleshooting](troubleshooting.md) |

Back up the data directory and music separately before upgrades or enabling
file management. Keep the database on local storage and run only one Rainy
process per data directory. The trash and edit history support recovery from
mistakes; NAS backups protect against disk failure.

Use [Privacy](../../PRIVACY.md) before sharing diagnostics, and the private
process in [SECURITY.md](../../SECURITY.md) for vulnerability reports.
