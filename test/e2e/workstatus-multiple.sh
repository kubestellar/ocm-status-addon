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
: Create a ManifestWork in hub to deliver two objects to cluster1
:
kubectl --context kind-hub apply -f - <<EOF
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  labels:
    managed-by.kubestellar.io/something: "true"
    app.kubernetes.io/name: nginx-deployment
  name: nginx-deployment
  namespace: cluster1
spec:
  workload:
    manifests:
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx
        namespace: default
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
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx-2
        namespace: default
      spec:
        replicas: 1
        selector:
          matchLabels:
            app: nginx-2
        template:
          metadata:
            labels:
              app: nginx-2
          spec:
            containers:
            - name: nginx-2
              image: public.ecr.aws/nginx/nginx:latest 
              ports:
              - containerPort: 80        
EOF

:
: -------------------------------------------------------------------------
: Verify that two WorkStatus has been created in the hub
:
wait-for-cmd '[ $(kubectl --context kind-hub -n cluster1 get workstatus -l app.kubernetes.io/name=nginx-deployment --no-headers | wc -l) == 2 ]'

:
: -------------------------------------------------------------------------
: Update the ManifestWork in hub to remove one object from cluster1
:
kubectl --context kind-hub apply -f - <<EOF
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  labels:
    managed-by.kubestellar.io/something: "true"
    app.kubernetes.io/name: nginx-deployment
  name: nginx-deployment
  namespace: cluster1
spec:
  workload:
    manifests:
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: nginx
        namespace: default
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
EOF

:
: -------------------------------------------------------------------------
: Verify that one WorkStatus has been deleted from the hub
:
wait-for-cmd '[ $(kubectl --context kind-hub -n cluster1 get workstatus -l app.kubernetes.io/name=nginx-deployment --no-headers | wc -l) == 1 ]'

