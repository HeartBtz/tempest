# Contributing

Tempest is intentionally scoped to authorised tracker testing in private labs.
Changes that add public exposure, authentication bypasses, peer traffic, payload
downloads, or support for unbounded inputs are out of scope.

## Development

Use Go 1.24, Node.js 24, npm, and a C compiler for SQLite. Install frontend
dependencies from the lockfile:

```sh
cd web
npm ci
cd ..
```

Before submitting a change, run:

```sh
gofmt -w cmd internal
go test ./...
go vet ./...
(cd web && npm run build && npm audit --audit-level=high)
docker compose config
docker compose build
```

Keep changes focused, add tests for security-sensitive behavior, do not include
real tracker URLs or passkeys in fixtures, screenshots, issues, or logs, and use
synthetic torrent metadata in examples.

## Reports

Use the private process in [SECURITY.md](SECURITY.md) for vulnerabilities. Use
the normal issue tracker for non-sensitive bugs and feature proposals.
