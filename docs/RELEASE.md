# Release and Relay contract

GitLab runs `validate` for merge request pipelines, the default branch and exact
SemVer tags (`vMAJOR.MINOR.PATCH`). A SemVer tag runs `build-release` only after
validation. The job refuses an unprotected tag through
`CI_COMMIT_REF_PROTECTED` and builds an archive whose binary and `/health`
response report the tag without its leading `v`.

Protect `v*` in GitLab and allow only Maintainers to create those tags. Relay
must treat the release as `semver_tag`, require `validate` and `build-release`,
and compare `status == "ok"` plus the exact `version` from `/health`.

The GitLab repository currently has no tags while the source version is 0.1.0.
Before enabling Relay release creation, establish the reviewed 0.1.0 commit as
the protected `v0.1.0` baseline so Relay does not infer an older `v0.0.1` release.

## Production deployment blocker

Repository and infrastructure records identify Tempest on VM127-Seedbox, but
they do not identify a dedicated CI receiver, forced command, deployment key,
production URL or receiver protocol. The repository therefore does not define a
`deploy-production` job. A generic SSH command, interactive root access or the
administrative Teleport identity must not be substituted.

Before enabling Relay deployment, provision and document all of the following:

- a dedicated source-limited forced receiver on VM127, with no interactive shell;
- a repository-defined immutable bundle/checksum protocol and rollback behavior;
- a manual, tag-only `deploy-production` job serialized by
  `resource_group: tempest-production` and protected `production` environment;
- a health URL reachable by Relay that returns the deployed exact SemVer;
- independent target-side service, version and health verification.

Until those controls exist, Relay may validate merge requests and release tags
but production deployment must remain blocked. Leave the project's Relay
deployment configuration unset: Relay requires a real manual production job and
must not be pointed at a placeholder.
