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

SCRIPT_HOME=$(dirname $0)

:
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
: Run the load driver
:
kubectl config use-context kind-hub
go run ${SCRIPT_HOME}/load-driver.go --num-objects 20


:
: -------------------------------------------------------------------------
: Verify that the expected number of work status objects has been created in the ITS
:
wait-for-cmd '(($(kubectl --context kind-hub -n cluster1 get workstatus --no-headers 2>/dev/null | wc -l) == 20))'

:
: -------------------------------------------------------------------------
: SUCCESS: Workstatus stress test passed
:
