# Status-AddOn

Agent and Controller built with the Open Cluster Management Add-On Framework
to provide deep status for selected resources.

# TL;DR

TODO


## Prereqs
- go version 1.20 and up
- make
- kubectl
- docker (or compatible docker engine that works with kind)
- kind
- helm

## Setup

1. Clone this repo.

2. Setup KSLight as described in [setup](https://github.ibm.com/dettori/kslight#setup), then deploy with:
    ```shell
    make install
    make deploy
    ```

3. Deploy a workload with KSLight, for example following [Scenario 1](https://github.ibm.com/dettori/kslight#scenario-1---multi-cluster-workload-deployment-with-kubectl)

3. Verify that `WorkStatus` objects are created in `imbs1` managed clusters namespaces 
    ```shell
    kubectl --context imbs1 get workstatuses -n cluster1 
    kubectl --context imbs1 get workstatuses -n cluster2
    ```

4. Inspect one of the statuses (change `WS_NAME` based on your scenario). You should be able to see a full status.
    ```shell
    WS_NAME=appsv1-deployment-nginx-nginx-deployment
    kubectl --context imbs1 get workstatuses -n cluster1 ${WS_NAME} -o yaml
    ```
