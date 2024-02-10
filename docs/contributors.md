# Developing Status-AddOn

## Run add-on agent in dev mode

```shell
make run-agent
```

## Build add-on image and push to docker registry

```shell
make ko-build-push
```

## Build and push chart

```shell
make chart-push
```

## Install chart from OCI repo

```shell
helm --kube-context imbs1 upgrade --install status-addon -n open-cluster-management oci://ghcr.io/kubestellar/ocm-status-addon-chart --version 0.2.0-alpha.1
```

## Install chart from local repo

```shell
helm --kube-context imbs1 upgrade --install status-addon -n open-cluster-management chart/ 
```

## Restart add-on manager on hub

```shell
kubectl --context imbs1 -n open-cluster-management scale deployment addon-status-controller --replicas 0
kubectl --context imbs1 -n open-cluster-management scale deployment addon-status-controller --replicas 1
```

## Check status of agent deployments

```shell
kubectl --context imbs1 -n cluster1 get managedclusteraddons addon-status
kubectl --context imbs1 -n cluster2 get managedclusteraddons addon-status
```

## Check workstatuses on hub

```shell
kubectl --context imbs1 get workstatuses -n cluster1 
kubectl --context imbs1 get workstatuses -n cluster2
```

## Delete workstatuses from hub (for debugging agent)

```shell
kubectl --context imbs1 delete workstatuses -n cluster1 --all
kubectl --context imbs1 delete workstatuses -n cluster2 --all
```

## Restart agents

```shell
kubectl --context cluster1 -n open-cluster-management-agent-addon scale deployment status-agent --replicas 0
kubectl --context cluster1 -n open-cluster-management-agent-addon scale deployment status-agent --replicas 1
kubectl --context cluster2 -n open-cluster-management-agent-addon scale deployment status-agent --replicas 0
kubectl --context cluster2 -n open-cluster-management-agent-addon scale deployment status-agent --replicas 1
```

## Uninstall addon

If installed with helm

```shell
helm --kube-context imbs1 -n open-cluster-management delete status-addon
```

If installed with `make deploy`

```shell
make undeploy
```

## Stop deployed agent on cluster1 (for local testing)

```shell
kubectl --context imbs1 -n open-cluster-management scale deployment addon-status-controller --replicas 0
```

Then change the number of replicas to 0 in the manifestwork for the addon by opening an editor.

```shell
kubectl --context imbs1 -n cluster1 edit manifestwork addon-addon-status-deploy-0 
```
and set spec.workload.manifests[0].spec.replicas = 0

Verify that the status agent was stopped:

```shell
kubectl --context cluster1 -n open-cluster-management-agent-addon get deployments.apps status-agent 
```
now you can start for local testing with `make run-agent`




