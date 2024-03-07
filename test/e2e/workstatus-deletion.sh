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
: Verify that the WorkStatus exists in the hub
:
wait-for-cmd '[ "$(kubectl --context kind-hub -n cluster1 get workstatus appsv1-deployment-nginx-nginx -o name --no-headers 2>/dev/null | wc -l)" == 1 ]'

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
