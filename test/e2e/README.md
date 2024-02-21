## Prerequisites

Almost the same as the [prerequisites specified by the project's README.md](https://github.com/kubestellar/ocm-status-addon/blob/main/README.md#prereqs).

The only exceptions are
- `clusteradm` is needed to run the test;
- `helm` is *not* needed to run the test.

## Use the tests

In the root directory of this git repo:
```
test/e2e/run.sh
```

Cleaning up:
```
test/e2e/cleanup.sh
```
