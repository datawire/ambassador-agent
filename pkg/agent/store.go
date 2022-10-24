package agent

import (
	"fmt"

	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	v1 "k8s.io/api/core/v1"
)

type configMapStore struct {
	sotw map[string]*kates.ConfigMap
}

type deploymentStore struct {
	sotw map[string]*kates.Deployment
}

type podStore struct {
	sotw map[string]*kates.Pod
}

type endpointStore struct {
	sotw map[string]*kates.Endpoints
}

// NewPodStore will create a new podStore filtering out undesired resources.
func NewPodStore(pods []*kates.Pod) *podStore {
	sotw := make(map[string]*kates.Pod)
	store := &podStore{sotw: sotw}

	for _, pod := range pods {
		if allowedNamespace(pod.GetNamespace()) && pod.Status.Phase != v1.PodSucceeded {
			key := fmt.Sprintf("%s.%s", pod.GetName(), pod.GetNamespace())
			store.sotw[key] = pod
		}
	}
	return store
}

// NewConfigMapStore will create a new configMapStore filtering out undesired resources.
func NewConfigMapStore(cms []*kates.ConfigMap) *configMapStore {
	sotw := make(map[string]*kates.ConfigMap)
	store := &configMapStore{sotw: sotw}

	for _, cm := range cms {
		if allowedNamespace(cm.GetNamespace()) {
			key := fmt.Sprintf("%s.%s", cm.GetName(), cm.GetNamespace())
			store.sotw[key] = cm
		}
	}
	return store
}

// NewDeploymentStore will create a new deploymentStore filtering out undesired resources.
func NewDeploymentStore(ds []*kates.Deployment) *deploymentStore {
	sotw := make(map[string]*kates.Deployment)
	store := &deploymentStore{sotw: sotw}

	for _, d := range ds {
		if allowedNamespace(d.GetNamespace()) {
			key := fmt.Sprintf("%s.%s", d.GetName(), d.GetNamespace())
			store.sotw[key] = d
		}
	}
	return store
}

// NewEndpointsStore will create a new endpointStore filtering out undesired resources.
func NewEndpointsStore(es []*kates.Endpoints) *endpointStore {
	sotw := make(map[string]*kates.Endpoints)
	store := &endpointStore{sotw: sotw}

	for _, ep := range es {
		if allowedNamespace(ep.GetNamespace()) {
			key := fmt.Sprintf("%s.%s", ep.GetName(), ep.GetNamespace())
			store.sotw[key] = ep
		}
	}
	return store
}

// StateOfWorld returns the current state of all pods from the allowed namespaces.
func (store *podStore) StateOfWorld() []*kates.Pod {
	pods := []*kates.Pod{}
	for _, v := range store.sotw {
		pods = append(pods, v)
	}
	return pods
}

func (store *endpointStore) StateOfWorld() []*kates.Endpoints {
	eps := []*kates.Endpoints{}
	for _, ep := range store.sotw {
		eps = append(eps, ep)
	}
	return eps
}

// StateOfWorld returns the current state of all configmaps from the allowed namespaces.
func (store *configMapStore) StateOfWorld() []*kates.ConfigMap {
	configs := []*kates.ConfigMap{}
	for _, v := range store.sotw {
		configs = append(configs, v)
	}
	return configs
}

// StateOfWorld returns the current state of all deployments from the allowed namespaces.
func (store *deploymentStore) StateOfWorld() []*kates.Deployment {
	deployments := []*kates.Deployment{}
	for _, v := range store.sotw {
		deployments = append(deployments, v)
	}
	return deployments
}

// allowedNamespace will check if resources from the given namespace
// should be reported to Ambassador Cloud.
func allowedNamespace(namespace string) bool {
	return namespace != "kube-system"
}
