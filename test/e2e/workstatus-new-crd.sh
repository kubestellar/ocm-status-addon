#!/usr/bin/env bash
# Copyright 2024 The KubeStellar Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -x # echo so that users can understand what is happening
set -e # exit on error

: -------------------------------------------------------------------------
: Delete all manifestworks except the add-on deployment
:
manifests=$(kubectl --context kind-hub -n cluster1 get manifestworks --no-headers -o custom-columns=":metadata.name")
for manifest in $manifests; do
  if [ "$manifest" != "addon-addon-status-deploy-0" ]; then
    kubectl --context kind-hub -n cluster1 delete manifestwork "$manifest"
  fi
done

:
: -------------------------------------------------------------------------
: Apply a new CRD and related clusterrole/clusterrolebinding in cluster1 
:
kubectl --context kind-cluster1 apply -f - <<EOF
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.kubestellar.io
spec:
  group: kubestellar.io
  names:
    kind: Foo
    listKind: FooList
    plural: foos
    singular: foo
  scope: Namespaced
  versions:
  - name: v1alpha1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            type: string
          kind:
            type: string
          metadata:
            type: object
          spec:
            type: object
            x-kubernetes-map-type: atomic
            x-kubernetes-preserve-unknown-fields: true
          status:
            type: object
            x-kubernetes-map-type: atomic
            x-kubernetes-preserve-unknown-fields: true
        required:
        - metadata
        - spec
        type: object
    served: true
    storage: true
    subresources:
      status: {}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: klusterlet-foo-access
rules:
- apiGroups: ["kubestellar.io"]
  resources: ["foos"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: klusterlet-foo-access
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: klusterlet-foo-access
subjects:
- kind: ServiceAccount
  name: klusterlet-work-sa
  namespace: open-cluster-management-agent
EOF

:
: -------------------------------------------------------------------------
: Create a ManifestWork in hub to deliver an instance of the CR to cluster1
:
kubectl --context kind-hub apply -f - <<EOF
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  name: foo
  namespace: cluster1
  labels:
    managed-by.kubestellar.io/something: "true"
    app.kubernetes.io/name: nginx-deployment
spec:
  workload:
    manifests:
    - apiVersion: kubestellar.io/v1alpha1
      kind: Foo
      metadata:
        name: myfoo
        namespace: default
      spec:
        arguments: {}
        entrypoint: whalesay 
EOF

:
: -------------------------------------------------------------------------
: Wait for object to show up in cluster1 
:
wait-for-cmd '[ $(kubectl --context kind-cluster1 get foos --no-headers | wc -l) == 1 ]'


:
: -------------------------------------------------------------------------
: Patch status
:
kubectl --context kind-cluster1 patch foos myfoo --type merge --subresource=status --patch='
status:
  artifactGCStatus:
    notSpecified: true
  artifactRepositoryRef:
    artifactRepository:
      archiveLogs: true
'

:
: -------------------------------------------------------------------------
: Verify that one WorkStatus has been created in the hub
:
wait-for-cmd '[ $(kubectl --context kind-hub -n cluster1 get workstatus --no-headers | wc -l) == 1 ]'

:
: -------------------------------------------------------------------------
: SUCCESS: Workstatus new CRD test passed
: