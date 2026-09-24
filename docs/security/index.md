# Security Documentation

Security guidance is split by audience:

- [Security policy](../../SECURITY.md): supported versions, private
  vulnerability reporting, scope, and disclosure.
- [Deployment security](../operations/security.md): network exposure, accounts
  and roles, sessions, Subsonic credentials, stored secrets, and container
  boundaries.
- [Reverse proxy and HTTPS](../operations/reverse-proxy.md): TLS termination,
  proxy trust, and the settings Rainy needs behind a proxy.
- [Secure development](../development/security.md): trust boundaries,
  filesystem rules, authentication, dependencies, and review requirements.
- [Privacy and data handling](../../PRIVACY.md): what Rainy stores, where it
  stores it, and what not to share.

Use the security policy to report a vulnerability. Use deployment security
when operating an instance, and secure development when changing the codebase.
