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
: Create hub cluster
:
kind create cluster --name hub
clusteradm init 1>/dev/null

:
: -------------------------------------------------------------------------
: Create managed cluster and register with hub
:
function create_and_register() {
  cluster=$1
  kind create cluster --name $cluster
  wait-for-cmd '[ $(kubectl --context kind-hub get ns open-cluster-management-hub -oname 2>/dev/null | wc -l) -eq 1 ]'
  kubectl --context kind-hub -n open-cluster-management-hub wait --for=condition=available deploy cluster-manager-registration-controller --timeout=120s
  kubectl --context kind-hub -n open-cluster-management-hub wait --for=condition=available deploy cluster-manager-registration-webhook --timeout=120s
  clusteradm --context kind-hub get token | grep '^clusteradm join' | sed "s/<cluster_name>/${cluster}/" | awk '{print $0 " --context 'kind-${cluster}' --force-internal-endpoint-lookup"}' | sh
}
create_and_register cluster1

:
: -------------------------------------------------------------------------
: Accept managed cluster
:
wait-for-cmd '[ $(kubectl --context kind-hub get csr 2>/dev/null | egrep ^cluster1- | grep -c Pending) -eq 1 ]'
clusteradm --context kind-hub accept --clusters cluster1
kubectl --context kind-hub get managedclusters

:
: -------------------------------------------------------------------------
: Build and deploy the addon
:
make ko-local-build
make kind-load-image CLUSTERS="hub cluster1"
make deploy DEFAULT_IMBS_CONTEXT=kind-hub CHART_INSTALL_EXTRA_ARGS="--set agent.v=5 --set agent.hub_burst=7"
git restore config/manager/kustomization.yaml # restore newTag
:
: -------------------------------------------------------------------------
: Wait for the agent to appear and come up
:
wait-for-cmd 'kubectl --context kind-cluster1 get deploy -n open-cluster-management-agent-addon status-agent'
kubectl --context kind-cluster1 wait deploy -n open-cluster-management-agent-addon status-agent --for condition=Available --timeout 180s

