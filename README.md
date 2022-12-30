# The Ambassador Agent

The Ambassador Agent is an optional compenent included with [Telepresence](https://github.com/telepresenceio/telepresence) and [Ambassador Edge Stack](https://github.com/emissary-ingress/emissary).
The Ambassador Agent securely reports snapshots of your cluster to [Ambassador Cloud](https://www.getambassador.io/products/ambassador-cloud/), which populate the service catalog giving you a birds-eye-view of your cluster and its services.
The Ambassador Agent provides a gRPC API to allow Telepresence to ask questions related to ingress resolution. 

## Installation

The Ambassador Agent can be installed via [Helm](https://helm.sh) (note: this will not install any other Ambassador products):
```shell
helm repo add datawire https://getambassador.io/
```
If you already have an account with Ambassador and a valid cloud token, the Ambassador Agent can be installed with the token in a single command:
```shell
helm install ambassador-agent datawire/ambassador-agent --namespace ambassador --create-namespace --set cloudConnectToken=<TOKEN>
```

If you would rather install the Ambassador Agent now and provide a token at a later time:
```
helm install ambassador-agent datawire/ambassador-agent --namespace ambassador --create-namespace
```

### Namespace-scoped installation

By default, the Ambassador Agent is installed with cluster-wide RBAC permissions.
If you would like to do a namespace-scoped installation, the namespaces that you would like the Ambassador Agent to snapshot can be passed in by adding them to `rbac.namespaces` in the `values.yaml` file.
```shell
helm install ambassador-agent datawire/ambassador-agent --namespace ambassador --create-namespace --set "rbac.namespace={<NAMESPACE_1>[,...]}"
```

## What gets collected in the snapshots?

In order to populate the and provided functionality when integrating with other Ambassador products, the Ambassador Agent requires the following permissions:
```yaml
- apiGroups: [ "" ]
  resources: [ "pods" ]
  verbs: [ "get", "list", "watch" ]
- apiGroups: [ "apps", "extensions" ]
  resources: [ "deployments" ]
  verbs: [ "get", "list", "watch" ]
- apiGroups: [ "" ]
  resources: [ "endpoints", "services" ]
  verbs: [ "get", "list", "watch" ]
- apiGroups: [ "" ]
  resources: [ "configmaps" ]
  verbs: [ "get", "list", "watch" ]
- apiGroups: [ "" ]
  resources: [ "namespaces" ]
  resourceName: [ "default" ]
  verbs: [ "get" ]
- apiGroups: [ "" ]
  resources: [ "endpoints", "services" ]
  verbs: [ "get", "list", "watch" ]
  ```
  
  To show information regarding argo, the following additional permissions are needed:
  ```yaml
- apiGroups: [ "argoproj.io" ]
  resources: [ "rollouts", "rollouts/status" ]
  verbs: [ "get", "list", "watch", "patch" ]
- apiGroups: [ "argoproj.io" ]
  resources: [ "application" ]
  verbs: [ "get", "list", "watch" ]
  ```
  
## Publish Mechanism

We use the [publish](.github/workflows/publish.yaml) action to
publish new versions of the extensions whenever a new tag is added to the
repo.

Images are pushed to [ambassador/ambassador-agent](https://hub.docker.com/repository/docker/ambassador/ambassador-agent).
We use multi-arch docker builds since the images need to be supported on
amd64 and arm64 machines, for more information on multi-arch docker builds
you can take a look at this
[dockerpage](https://www.docker.com/blog/multi-arch-build-and-images-the-simple-way/)

To trigger the publish workflow, run the following commands:

```
git tag --annotate --message='Releasing version vSEMVER' vSEMVER
git push origin vSEMVER
```

You can then follow along in the [actions tab](https://github.com/datawire/ambassador-agent/actions)
