## Prerequisites

Almost the same as the [prerequisites specified by the project's README.md](https://github.com/kubestellar/ocm-status-addon/blob/main/README.md#prereqs).

The only exceptions are
- `clusteradm` is needed to run the test;
- `helm` is *not* needed to run the test.

## What are the tests

- [workstatus-patching.sh](./workstatus-patching.sh)
  ensures that the status addon can always patch WorkStatus objects, no matter the '.status' field is empty or not. This test is created for [issue #22](https://github.com/kubestellar/ocm-status-addon/issues/21).
- [workstatus-crud.sh](./workstatus-crud.sh)
  verifies the status addon's CRUD operations on WorkStatus objects.
- [workstatus-multiple.sh](./workstatus-multiple.sh)
  verifies the status handles correctly creation and deletion of WorkStatus objects when objects are part of the same ManifestWork.
- [workstatus-stress-test.sh](./workstatus-stress-test.sh) 
  creates, deletes and recreates N objects without pause between each operation and then checks all workstatuses are 
  present on the hub.

## How to use the tests

In the root directory of this git repo:
```
test/e2e/run.sh
```

Cleaning up:
```
test/e2e/cleanup.sh
```
