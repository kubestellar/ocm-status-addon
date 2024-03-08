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

:
: -------------------------------------------------------------------------
: Create a ManifestWork in hub to deliver an app to cluster1
:
kubectl --context kind-hub apply -f - <<EOF
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  labels:
    managed-by.kubestellar.io/something: "true"
  name: nginx-deployment
  namespace: cluster1
spec:
  workload:
    manifests:
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx
        namespace: nginx
      spec:
        replicas: 1
        selector:
          matchLabels:
            app: nginx
        template:
          metadata:
            labels:
              app: nginx
          spec:
            containers:
            - name: nginx
              image: public.ecr.aws/nginx/nginx:latest 
              ports:
              - containerPort: 80
---
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  name: nginx-namespace
  namespace: cluster1
spec:
  workload:
    manifests:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: nginx
EOF

:
: -------------------------------------------------------------------------
: Verify that the WorkStatus has been created in the hub
:
wait-for-cmd '[ "$(kubectl --context kind-hub -n cluster1 get workstatus appsv1-deployment-nginx-nginx -o jsonpath="{.status.availableReplicas}" 2>/dev/null)" == 1 ]'

:
: -------------------------------------------------------------------------
: SUCCESS: Workstatus creation test passed
:

:
: -------------------------------------------------------------------------
: Update the ManifestWork object in hub by scaling to zero replicas
:
kubectl --context kind-hub -n cluster1 patch manifestwork nginx-deployment \
  --type json \
  --patch '[{"op": "replace", "path": "/spec/workload/manifests/0/spec/replicas", "value": 0}]'

:
: -------------------------------------------------------------------------
: Verify that the WorkStatus has been updated in the hub
:
wait-for-cmd '[ -z "$(kubectl --context kind-hub -n cluster1 get workstatus appsv1-deployment-nginx-nginx -o jsonpath="{.status.availableReplicas}" 2>/dev/null)" ]'

:
: -------------------------------------------------------------------------
: Update the ManifestWork object in hub by scaling to two replicas
:
kubectl --context kind-hub -n cluster1 patch manifestwork nginx-deployment \
  --type json \
  --patch '[{"op": "replace", "path": "/spec/workload/manifests/0/spec/replicas", "value": 2}]'

:
: -------------------------------------------------------------------------
: Verify that the WorkStatus has been updated in the hub
:
wait-for-cmd '[ "$(kubectl --context kind-hub -n cluster1 get workstatus appsv1-deployment-nginx-nginx -o jsonpath="{.status.availableReplicas}" 2>/dev/null)" == 2 ]'

:
: -------------------------------------------------------------------------
: SUCCESS: Workstatus updating test passed
:

:
: -------------------------------------------------------------------------
: Delete the ManifestWork object in hub
:
kubectl --context kind-hub -n cluster1 delete manifestwork nginx-deployment

:
: -------------------------------------------------------------------------
: Verify that the WorkStatus has been deleted in the hub
:
wait-for-cmd '[ "$(kubectl --context kind-hub -n cluster1 get workstatus appsv1-deployment-nginx-nginx -o name --no-headers 2>/dev/null | wc -l)" == 0 ]'

:
: -------------------------------------------------------------------------
: SUCCESS: Workstatus deletion test passed
:
