# Status AddOn Relesea process 

## Steps to make release

1. Checkout main and fetch from upstream
2. git checkout <release branch> # e.g. release-0.2
3. Rebase from main
4. Push the release branch with "git push" - open PR and review/merge to update release branch upstream.

5. check existing tags e.g.,
```
git tag 
v0.2.1
```
6. create a new tag e.g.
```
git tag v0.2.2
```
7. Push the tag upstream
```
git push upstream --tag v0.2.2
```