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
