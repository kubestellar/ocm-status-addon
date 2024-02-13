# Status AddOn Relesea process 

The Status AddOn release process is based on [GoReleaser](https://goreleaser.com).
GoReleaser is configured to automatically create a new release by pushing new tags matching the
pattern 'v*' and build and publish to ghcr.io the packages for the 
[container image](https://github.com/kubestellar/ocm-status-addon/pkgs/container/ocm-status-addon)
and the [helm chart](https://github.com/kubestellar/ocm-status-addon/pkgs/container/ocm-status-addon-chart) 
used to deploy the status add-on. The relevant files for goreleaser configuration are
[./goreleaser.yaml](../.goreleaser.yaml) and [./.github/workflows/goreleaser.yml](../.github/workflows/goreleaser.yml).

The steps outlined below assume that a release branch for the release that is going to be
created already exists. Typically a release branch is created at the beginning of each
new release cycle with `git checkout -b <release branch>`.

## Steps to make release

1. Fetch from upstream and checkout main:
```shell
git fetch upstream
git checkout main
```
2. Rebase into main
```shell
git rebase upstream/main
```
3. Checkout latest release branch
```shell
git checkout <release branch> # e.g. release-0.2
```
4. Rebase into latest release branch
```shell
git rebase main
```
5. Push the release branch
```shell
git push
```
6. Open PR and review/merge to update release branch upstream

7. check existing tags e.g.,
```shell
git tag 
v0.2.1
```
8. create a new tag e.g.
```shell
git tag v0.2.2
```
9. Push the tag upstream
```shell
git push upstream --tag v0.2.2
```

Go releaser will automatically create a new release and publish the release artifacts
(container image and helm chart) under https://github.com/orgs/kubestellar/packages.