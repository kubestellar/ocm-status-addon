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
: Build and deploy the addon
:
make ko-local-build
make kind-load-image CLUSTERS="hub cluster1"
make deploy DEFAULT_IMBS_CONTEXT=kind-hub
git restore config/manager/kustomization.yaml # restore newTag

:
: -------------------------------------------------------------------------
: Create a ManifestWork in hub to deliver an app to cluster1
:
kubectl --context kind-hub apply -f - <<EOF
apiVersion: work.open-cluster-management.io/v1
kind: ManifestWork
metadata:
  labels:
    transport.kubestellar.io/something: wds1
  name: nginx-deployment
  namespace: cluster1
spec:
  workload:
    manifests:
    - apiVersion: v1
      kind: Namespace
      metadata:
        labels:
          managed-by.kubestellar.io/singletonstatus: bindingpolicy
        name: nginx
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        labels:
          managed-by.kubestellar.io/singletonstatus: bindingpolicy
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
