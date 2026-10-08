# Development

Start with [Local development](local-dev.md), then use the smallest Makefile
target that covers the change. Run `make` or `make help` for common commands.

| Task | Guide |
| --- | --- |
| Set up and run a synthetic library | [Local development](local-dev.md) |
| Change Go services or HTTP handlers | [Backend guidelines](backend-guidelines.md) |
| Change React features and shared UI | [Frontend guidelines](frontend-guidelines.md) |
| Review layout, themes, and playback | [Design](design.md) |
| Choose and run tests | [Testing](testing.md) |
| Understand Actions job selection | [CI and release automation](ci.md) |
| Change persistence | [Migrations](migrations.md) |
| Change a trust boundary | [Secure development](security.md) |
| Prepare a signed commit or release | [Commit and release](commit-and-release.md) |

Before changing behavior, read [Core boundaries](../architecture/core-boundaries.md)
and the relevant sections of the [contract](../architecture/contract.md).
Update those sections with the code and explain observable behavior in the
[User guide](../user/index.md) or [Operations](../operations/index.md).

Contributor rules live in [CONTRIBUTING.md](../../CONTRIBUTING.md); agent
instructions live in [AGENTS.md](../../AGENTS.md). Release notes record changes,
while these guides describe the current workflow.
