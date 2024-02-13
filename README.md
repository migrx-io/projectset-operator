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

2. Create git repo to host CRDs (Optional) 

    To support GitOps approach you can create git repo 

    Example structure https://github.com/migrx-io/projectset-crds.git

    ```

    ├── common-templates
    ├── prod-ocp                           # env declaration
    │   └── test-ocp
    │       ├── crds                       # env crds/projectsets
    │       └── templates                  # env templates/projectsettemplates
    ...
    ├── projectsets.yaml                   # env metadata (define here envs)
    ...
    └── test-ocp                           # env declarations
        ├── crds
        │   ├── dev-app-template.yaml
        │   └── dev-app.yaml
        └── templates
            └── dev-small.yaml

    ```

    Define cluster env in **projectsets.yaml** in roor git repo

    ```
    envs:
    test-ocp-cluster:                                # env name/alias
        projectset-templates: test-ocp/templates       # path to templates dir
        projectset-crds: test-ocp/crds                 # path to crds dir
    ...
    prod-ocp-cluster:
        projectset-templates: common-templates
        projectset-crds: prod-ocp/crds

    ```

3. Create secret with GitHub/GitLab token (for ProsetSetSync to sync CRDs from git repo)

    ```sh
    kubectl create secret generic projectsetsync-secret \                                        
        --namespace projectset-operator-system \                                                          
        --from-literal=token=<base64(GIT_TOKEN)>

    ```

4. Create CRD ProjectSet/ProjectSetTemplate (without GitOps)

    See examples here https://github.com/migrx-io/projectset-operator/tree/main/config/samples

    ```sh
    kubectl apply -f <PATH to YAML>

    ```

4. Create CRD ProjectSet/ProjectSetTemplate (GitOps)

    Create ProjectSync configuration

    Example

    ```

    apiVersion: project.migrx.io/v1alpha1
    kind: ProjectSetSync
    metadata:
    labels:
        app.kubernetes.io/name: projectsetsync
        app.kubernetes.io/instance: projectsetsync-instance
        app.kubernetes.io/part-of: projectset-operator
        app.kubernetes.io/managed-by: kustomize
        app.kubernetes.io/created-by: projectset-operator
    name: test-ocp-cluster
    spec:
    gitRepo: https://github.com/migrx-io/projectset-crds.git    # git uri
    envName: test-ocp-cluster                                   # env name from projectsets.yaml (see git repo structure)
    gitBranch: main                                             # branch name
    syncSecInterval: 10                                         # pull sync interval
    confFile: projectsets.yaml                                  # path to git config env file  (by default projectsets.yaml)


    ```

    ```sh
    kubectl apply -f <PATH to YAML>

    ```

    When you push CRDs to repo (crds/template) operator will sync it and create resources


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

