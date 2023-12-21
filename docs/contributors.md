# Developing Status-AddOn

TODO - update all sections

## Build and push manually image

Use GH personal access token with write to packages permissions to login to ghcr.io

```shell
docker login ghcr.io -u USERNAME 
```

```shell
export KO_DOCKER_REPO=ghcr.io/kubestellar/kubeflex
ko build -B ./cmd/status-addon-operator -t 0.1.0 --platform linux/amd64,linux/arm64
```

Image is pushed to `ghcr.io/kubestellar/kubeflex/status-addon-operator:0.1.0`

## Build and push manually helm chart

```shell
make chart
helm package ./chart --destination . --version 0.1.0
helm push ./*.tgz oci://ghcr.io/kubestellar/kubeflex
```

## Install the chart

```shell
helm upgrade --install status-addon -n <namespace> oci://ghcr.io/kubestellar/kubeflex/status-addon-operator-chart --version 0.1.0
```

## Build image and deploy status-addon on kind

```shell
kubectl config use-context kind-kubeflex
make docker-build
kind load docker-image ghcr.io/kubestellar/kubeflex/status-addon-operator:0.1.0 --name kubeflex
make deploy
```

## Run addon agent for local testing

This requires everything setup and imbs1 and cluster1 up and running

```shell
make run-addon-agent
```

## Hacks

### Minify kubeconfig for a specified context

```shell
kubectl config view --minify --context=cp2 --flatten > ${HOME}/.ocm.kubeconfig
```

### Show all resources in a namespace

```shell
kubectl api-resources --verbs=list --namespaced -o name   | xargs -n 1 kubectl get --show-kind --ignore-not-found -n <namespace>
```


