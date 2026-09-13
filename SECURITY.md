# Security Policy

## Deployment boundary

Tempest is an unauthenticated tracker-announce test tool. Bind it to loopback
by default. If remote access is needed, put it behind a private authenticated
proxy and restrict access to authorised operators.

Never expose Tempest directly to the Internet or use it with trackers you are
not authorised to test. Uploaded torrent metadata and activity logs can be
sensitive operational data.

The default outbound policy rejects loopback, private, link-local, unspecified,
and multicast tracker destinations, including redirect targets. An isolated lab
that intentionally uses an authorised private tracker may set
`TEMPEST_ALLOW_PRIVATE_TRACKERS=true`. This is an explicit SSRF-policy bypass:
do not enable it on shared or Internet-accessible instances.

The application does not provide user authentication, TLS termination, tenant
isolation, or authorization. The Docker Compose defaults publish only on
`127.0.0.1`, run as a non-root user with all capabilities dropped, and use a
read-only root filesystem. Preserve those controls when adapting deployment.

Tracker URLs are redacted from application logs, but torrent names, comments,
tracker responses, and historical databases may still contain sensitive data.
Do not attach real databases, torrent files, or full logs to public reports.

## Supported versions

Security fixes are provided for the current default branch. This project has
not yet published a stable release support matrix.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Report it privately
to the repository owner with the affected version, a minimal reproduction and
the expected impact.

Include the affected commit, deployment mode, reproduction steps using
synthetic data, and whether the default security configuration is affected.
Do not test against systems or trackers you do not own or have permission to
assess. The maintainer will acknowledge a complete report when available and
coordinate disclosure after a fix is ready.
