# GitHub Actions Workflows

This directory defines CI/CD for cluster-bloom. Workflow file names follow the same convention as [cluster-forge](https://github.com/silogen/cluster-forge/tree/main/.github/workflows).

## Naming convention

| Pattern | Example | When to use |
|---------|---------|-------------|
| `pull-request-*.yml` | `pull-request-checks.yml` | Validation that runs on pull requests |
| `integration-tests.yml` | `integration-tests.yml` | Long-running or integration test suites |
| `release.yml` | `release.yml` | Release builds, packaging, and publishing |

The `name:` field in each workflow uses the same vocabulary (for example **Pull Request Checks**, **Integration Tests**, **Release**) so both repositories look consistent in the GitHub Actions UI.

## Workflow map

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| [pull-request-checks.yml](./pull-request-checks.yml) | Pull requests to `main` | Documentation sync for `feat:`/`docs:` commits and devbox build smoke test |
| [integration-tests.yml](./integration-tests.yml) | Pull requests to `main`, manual | QEMU integration tests |
| [release.yml](./release.yml) | Release published, manual | Builds release binaries and uploads assets to GitHub Releases |

## Pull request validation

Every PR to `main` runs:

1. **Pull Request Checks** — Ensures `feat:` and `docs:` commits include updates under `docs/` or `README.md`, then verifies the project builds with devbox.
2. **Integration Tests** — Runs the QEMU-based integration test suite.

Neither PR workflow creates or modifies GitHub Releases.

## Release process

### Recommended: draft and publish in GitHub

1. In GitHub, go to **Releases → Draft a new release**.
2. Set the tag (for example `v1.2.0-rc1`) and mark **Set as a pre-release** when appropriate.
3. Save the draft, review notes, then **Publish release**.
4. The **Release** workflow runs on `release: published`, builds binaries with the tag version injected, and uploads `dist/*` to the release.

### Alternative: manual workflow dispatch

Use **Actions → Release → Run workflow** when you want CI to calculate the next semver from conventional commits, create the GitHub Release, build, and upload assets.

- **is_prerelease** (default `true`): marks the created release as a prerelease.

## Adding new checks

- **PR-only validation** → add to an existing `pull-request-*.yml` workflow or create a new one following that naming pattern.
- **Release artifacts or publishing** → extend `release.yml`.
- **Long-running integration tests** → extend `integration-tests.yml`.

Do not combine PR validation and release publishing in one workflow.
