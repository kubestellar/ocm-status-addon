# Developing Status-AddOn

## Run add-on agent in dev mode

```shell
make run-agent
```

## Build add-on image and push

```shell
make ko-build-push
```

## Restart add-on manager on hub

```shell
kubectl --context imbs1 -n open-cluster-management scale deployment addon-status-controller --replicas 0
kubectl --context imbs1 -n open-cluster-management scale deployment addon-status-controller --replicas 1
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





