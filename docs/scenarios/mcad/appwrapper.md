# Delivering MCAD AppWrapper Objects 

First, make sure you are running the latest status-addon, which includes
a fix for CRDs delivery. If you already have a WDS, you may update
the status-addon controller to the latest image with the following procedure
(for wds1).

```shell
kubectl --context kind-kubeflex patch deployment status-addon-controller-manager --namespace wds1-system --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/1/imagePullPolicy", "value": "Always"}]'
sleep 5
kubectl --context kind-kubeflex patch deployment status-addon-controller-manager --namespace wds1-system --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/1/imagePullPolicy", "value": "IfNotPresent"}]'
```

There is curremtly an issue with using OCM Klusterlet to deploy a CR. The error that will show
up in manifestwork conditions is something like:

```
 'Failed to apply manifest: appwrappers.mcad.ibm.com "aw-wait20-40-test"
          is forbidden: User "system:serviceaccount:open-cluster-management-agent:klusterlet-work-sa"
          cannot get resource "appwrappers" in API group "mcad.ibm.com" in the namespace
          "test-kfp"'
```

A workaround until a better solution is found is to apply the following clusterrole/clusterrolebinding to cluster1
and cluster2:

Repeat the command below for 'cluster=cluster1' and 'cluster=cluster2'

```shell
kubectl --context ${cluster} apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: appwrappers-access
rules:
- apiGroups: ["mcad.ibm.com"]
  resources: ["appwrappers"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: klusterlet-appwrappers-access
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: appwrappers-access
subjects:
- kind: ServiceAccount
  name: klusterlet-work-sa
  namespace: open-cluster-management-agent
EOF  
```

Once that is done, you can apply the CRD, create the test-kfp namespace and the app wrapper on wds1.
(note: you may find a better CRD match for the supplied appwrapper, the current one required some change)

Update: https://open-cluster-management.io/concepts/manifestwork/ describe the issue in
"Permission setting for work agent" and provides two possible solutions:

1. add permission on the managed cluster directly (which is what we did above)

2. add permission on the hub cluster by another ManifestWork

In order to make this process transparent in status-addon, one approach would be to autogenerate 
the clusterrole for option 1 in status-addon and apply it. "aggregated clusterRole" seems the more
promising approache here.

```shell
curl -LO https://raw.githubusercontent.com/project-codeflare/multi-cluster-app-dispatcher/v1.33.0/config/crd/bases/mcad.ibm.com_appwrappers.yaml

kubectl --context wds1 apply -f mcad.ibm.com_appwrappers.yaml 

kubectl --context wds1 create ns test-kfp
```

Now you can apply the app wrapper, for example:

```shell
kubectl --context wds1 apply -f - <<EOF
apiVersion: mcad.ibm.com/v1beta1
kind: AppWrapper
metadata:
  name: aw-wait20-40-test
  namespace: test-kfp
spec:
  # schedulingSpec:
  #   minAvailable: 1
  #   requeuing:
  #       timeInSeconds: 120
  #       growthType: "exponential" 
  priority: 9
  resources:
    GenericItems:
    - replicas: 1
      # completionstatus: Complete
      completionstatus: Complete,Failed
      custompodresources:
      - replicas: 1
        requests:
          cpu: 500m
          memory: 512Mi
          nvidia.com/gpu: 0
        limits:
          cpu: 500m
          memory: 512Mi
          nvidia.com/gpu: 0
      generictemplate:
        apiVersion: batch/v1
        kind: Job
        metadata:
          namespace: test-kfp
          name: aw-wait-test40
          # labels:
          #   appwrapper.mcad.ibm.com: defaultaw-schd-spec-with-timeout-1
        spec:
          parallelism: 1
          completions: 1
          template:
            metadata:
              namespace: test-kfp
              labels:
                appwrapper.mcad.ibm.com: "aw-wait-test"
            spec:
              containers:
              - name: aw-wait-test40
                image: ubuntu:latest
                command: [ "/bin/bash", "-c", "--" ]
                args: [ "sleep 20" ]
                resources:
                  requests:
                    memory: "512Mi"
                    cpu: "500m"
                  limits:
                    memory: "512Mi"
                    cpu: "500m"
              restartPolicy: Never
  # schedulingSpec:
  #   clusterScheduling:
  #     clusters:
  #     - name: wec3
   # dispatchDuration: {}
    # requeuing:
    #   growthType: exponential
    #   maxNumRequeuings: 0
    #   maxTimeInSeconds: 0
    #   numRequeuings: 0
    #   timeInSeconds: 300
EOF    
```

Then, label all resources (CRD, namespace and appwrapper) with a label that is then referenced in the placement:

```shell
kubectl --context wds1 label crd appwrappers.mcad.ibm.com app.kubernetes.io/name=aw-wait20-40-test
kubectl --context wds1 label appwrappers -n test-kfp aw-wait20-40-test app.kubernetes.io/name=aw-wait20-40-test
kubectl --context wds1 label ns test-kfp app.kubernetes.io/name=aw-wait20-40-test 
```

Finally, apply the placement. Note that both clusters have the same label. The placement will deliver
to both. To deliver only to one, we can add a second label to one of the two clusters and use selectors
for both labels (which are AND'd together). With the setup in the example, status-addon delivers to both clusters.

```shell
kubectl --context wds1 apply -f - <<EOF
apiVersion: edge.kubestellar.io/v1alpha1
kind: Placement
metadata:
  name: nginx-placement
spec:
  clusterSelectors:
  - matchLabels: {"location-group":"edge"}
  downsync:
  - labelSelectors:
    - matchLabels: {"app.kubernetes.io/name":"aw-wait20-40-test"}
EOF
```

Check that the app wrapper has been delivered:

```shell
kubectl --context cluster1 get appwrappers.mcad.ibm.com -n test-kfp aw-wait20-40-test
kubectl --context cluster2 get appwrappers.mcad.ibm.com -n test-kfp aw-wait20-40-test 

```

