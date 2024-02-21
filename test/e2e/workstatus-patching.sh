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
: Create a WorkStatus object in hub
:
kubectl --context kind-hub apply -f - <<EOF
apiVersion: control.kubestellar.io/v1alpha1
kind: WorkStatus
metadata:
  labels:
    managed-by.kubestellar.io/singletonstatus: "true"
    managed-by.kubestellar.io/wds1.nginx-singleton-bindingpolicy: "true"
  name: test
  namespace: default
spec:
  sourceRef:
    group: apps
    kind: Deployment
    name: nginx-deployment
    namespace: nginx
    resource: deployments
    version: v1
EOF

:
: -------------------------------------------------------------------------
: Patch when there is no status set
:
kubectl --context kind-hub patch workstatus test --type=merge \
  --patch '{"metadata":{"annotations":{"version": "v2"}}}'
wait-for-cmd '[ $(kubectl --context kind-hub get workstatus test -o=jsonpath='{.metadata.annotations.version}') = "v2" ]'

:
: -------------------------------------------------------------------------
: Fill in then show the status
:
kubectl --context kind-hub patch workstatus test --type=merge \
  --patch='{"status":{"conditions":[{"type":"Available","status":"True","reason":"DeploymentAvailable","message":"Deployment has minimum availability."}]}}' \
  --subresource=status
kubectl --context kind-hub get workstatus test -o=jsonpath='{.status}'

:
: -------------------------------------------------------------------------
: Patch when there is some status set
:
kubectl --context kind-hub patch workstatus test --type=merge \
  --patch '{"metadata":{"annotations":{"version": "v3"}}}'
wait-for-cmd '[ $(kubectl --context kind-hub get workstatus test -o=jsonpath='{.metadata.annotations.version}') = "v3" ]'

:
: -------------------------------------------------------------------------
: SUCCESS: Workstatus patching test passed
:
