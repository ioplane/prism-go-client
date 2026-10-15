# Contributing

Contributions are always welcome. Before contributing please search the [issue tracker](../../issues). Your issue may have already been discussed or fixed in `main`. To contribute, [fork](https://help.github.com/articles/fork-a-repo/) this repository, commit your changes, and [send a Pull Request](https://help.github.com/articles/using-pull-requests/).

## Feature requests

Feature requests should be submitted in the [issue tracker](../../issues), with a description of the expected behavior & use case, where they'll remain closed until sufficient interest, [e.g. :+1: reactions](https://help.github.com/articles/about-discussions-in-issues-and-pull-requests/), has been [shown by the community](../../issues?q=label%3A%22votes+needed%22+sort%3Areactions-%2B1-desc).

Before submitting an issue, please search for similar ones in the [closed issues](../../issues?q=is%3Aissue+is%3Aclosed+label%3Aenhancement).

## Pull requests

### Approval and release process

Pull requests approvals go through the following steps:

1. A GitHub action may be triggered to lint and test.
2. A maintainer reviews the changes. Any change requires at least one review.
3. The pull request can be merged when at least one maintainer approves it.

## Contributor License Agreement

Before you submit your pull request, you'll need to sign the [Nutanix Contributor License Agreement (CLA)](https://www.nutanix.dev/cla/). The CLA must be agreed to by all contributors who are not Nutanix Full-time Employees (FTE) or interns prior to the contribution being merged into the project codebase. The CLA is substantially similar to the Apache Contributor License Agreement, which is the industry standard CLA.

For more information about CLAs, please check out Alex Russell's excellent post,
["Why Do I Need to Sign This?"](https://infrequently.org/2008/06/why-do-i-need-to-sign-this/).
## Fork contribution and release policy

The ioplane fork follows [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/)
and [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/).
Use technical Russian declarative noun phrases for new commit and PR titles:
`fix(http): безопасная диагностика`. Allowed types are `build`, `chore`, `ci`,
`docs`, `feat`, `fix`, `perf`, `refactor`, `revert`, `style`, and `test`.
Mark incompatible public API or minimum-toolchain changes with `!` and document
consumer migration in CHANGELOG and the compatibility contract.

Every user-visible, operational, policy or tooling change adds an impact entry
under `Unreleased`, grouped as Added, Changed, Deprecated, Removed, Fixed or
Security. New release headings use dated SemVer: `## [1.2.3] - YYYY-MM-DD`.
Published historical entries and inherited commits are preserved. The inherited
SDK release-header exceptions in `.ci/changelog-legacy.txt` do not authorize new
undated releases. Merge commits are excluded from title checks; PR titles and
all non-merge commits in the reviewed range are checked.

Run the governance target through the declared Podman environment. CI verifies
the same target. Checks are workflow requirements; branch protection and review
approval must be verified separately. CI success does not authorize merging,
registering a driver in Rancher or changing production VMs.

The detailed migration contract is in [the development plan](docs/ru/development-plan.md).
