# projectset-operator

The ProjectSet Operator creates/configures/manages K8s/OpenShift namespaces

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Install on cluster 

1. Install Operator and all CRs

```sh
kubectl apply -f https://raw.githubusercontent.com/migrx-io/projectset-operator/main/config/manifests.yaml
```

2. Create secret with GitHub/GitLab token (for ProsetSetSync to sync CRDs from git repo)

```sh
 kubectl create secret generic projectsetsync-secret \                                        
      --namespace projectset-operator-system \                                                          
      --from-literal=token=<GIT_TOKEN>

```


## Development

### Install CRDs
Install Operator Instances of Custom Resources:

```sh
make install
```

### Run operator locally
To run operator locally 

```sh
make run
```
### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make generate
make manifests
make helm  # generate new all-in-one manifests.yaml
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)


### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

## Build and deploy/undeploy

1. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/projectset-operator:tag
```

## License

Copyright 2024 Anatolii Makarov.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

