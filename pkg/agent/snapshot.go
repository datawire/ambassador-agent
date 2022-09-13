package agent

import (
	"encoding/json"

	amb "github.com/emissary-ingress/emissary/v3/pkg/api/getambassador.io/v3alpha1"
	"github.com/emissary-ingress/emissary/v3/pkg/consulwatch"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	types_ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	types_net_v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	gw "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

type Snapshot struct {
	// meta information to identify the ambassador
	AmbassadorMeta *AmbassadorMetaInfo
	// The Kubernetes field contains all the ambassador inputs from kubernetes.
	Kubernetes *KubernetesSnapshot
	// The Consul field contains endpoint data for any mappings setup to use a
	// consul resolver.
	Consul *ConsulSnapshot
	// The Deltas field contains a list of deltas to indicate what has changed
	// since the prior snapshot. This is only computed for the Kubernetes
	// portion of the snapshot. Changes in the Consul endpoint data are not
	// reflected in this field.
	Deltas []*Delta
	// The APIDocs field contains a list of OpenAPI documents scrapped from
	// Ambassador Mappings part of the KubernetesSnapshot
	APIDocs []*APIDoc `json:"APIDocs,omitempty"`
	// The Invalid field contains any kubernetes resources that have failed
	// validation.
	Invalid []*unstructured.Unstructured
	Raw     json.RawMessage `json:"-"`
}

type AmbassadorMetaInfo struct {
	ClusterID         string          `json:"cluster_id"`
	AmbassadorID      string          `json:"ambassador_id"`
	AmbassadorVersion string          `json:"ambassador_version"`
	KubeVersion       string          `json:"kube_version"`
	Sidecar           json.RawMessage `json:"sidecar"`
}

type KubernetesSnapshot struct {
	// k8s resources
	IngressClasses []*IngressClass     `json:"ingressclasses"`
	Ingresses      []*Ingress          `json:"ingresses"`
	Services       []*corev1.Service   `json:"service"`
	Endpoints      []*corev1.Endpoints `json:"Endpoints"`

	// ambassador resources
	Listeners   []*amb.Listener   `json:"Listener"`
	Hosts       []*amb.Host       `json:"Host"`
	Mappings    []*amb.Mapping    `json:"Mapping"`
	TCPMappings []*amb.TCPMapping `json:"TCPMapping"`
	Modules     []*amb.Module     `json:"Module"`
	TLSContexts []*amb.TLSContext `json:"TLSContext"`

	// plugin services
	AuthServices      []*amb.AuthService      `json:"AuthService"`
	RateLimitServices []*amb.RateLimitService `json:"RateLimitService"`
	LogServices       []*amb.LogService       `json:"LogService"`
	TracingServices   []*amb.TracingService   `json:"TracingService"`
	DevPortals        []*amb.DevPortal        `json:"DevPortal"`

	// resolvers
	ConsulResolvers             []*amb.ConsulResolver             `json:"ConsulResolver"`
	KubernetesEndpointResolvers []*amb.KubernetesEndpointResolver `json:"KubernetesEndpointResolver"`
	KubernetesServiceResolvers  []*amb.KubernetesServiceResolver  `json:"KubernetesServiceResolver"`

	// gateway api
	GatewayClasses []*gw.GatewayClass
	Gateways       []*gw.Gateway
	HTTPRoutes     []*gw.HTTPRoute

	// It is safe to ignore AmbassadorInstallation, ambassador doesn't need to look at those, just
	// the operator.

	KNativeClusterIngresses []*unstructured.Unstructured `json:"clusteringresses.networking.internal.knative.dev,omitempty"`
	KNativeIngresses        []*unstructured.Unstructured `json:"ingresses.networking.internal.knative.dev,omitempty"`

	FilterPolicies []*unstructured.Unstructured `json:"filterpolicies.v3alpha1.getambassador.io,omitempty"`
	Filters        []*unstructured.Unstructured `json:"filters.v3alpha1.getambassador.io,omitempty"`

	K8sSecrets []*corev1.Secret             `json:"-"`      // Secrets from Kubernetes
	FSSecrets  map[SecretRef]*corev1.Secret `json:"-"`      // Secrets from the filesystem
	Secrets    []*corev1.Secret             `json:"secret"` // Secrets we'll feed to Ambassador

	ConfigMaps []*corev1.ConfigMap `json:"ConfigMaps,omitempty"`

	// [kind/name.namespace][]kates.Object
	Annotations map[string][]Object `json:"annotations"`

	// Pods and Deployments were added to be used by Ambassador Agent so it can
	// report to AgentCom in Ambassador Cloud.
	Pods        []*corev1.Pod        `json:"Pods,omitempty"`
	Deployments []*appsv1.Deployment `json:"Deployments,omitempty"`

	// ArgoRollouts represents the argo-rollout CRD state of the world that may or may not be present
	// in the client's cluster. For this reason, Rollouts resources are fetched making use of the
	// k8s dynamic client that returns an unstructured.Unstructured object. This is a better strategy
	// for Ambassador code base for the following reasons:
	//   - it is forward compatible
	//   - no need to maintain types defined by the Argo projects
	//   - no unnecessary overhead Marshaling/Unmarshaling it into json as the state is opaque to
	// Ambassador.
	ArgoRollouts []*unstructured.Unstructured `json:"ArgoRollouts,omitempty"`

	// ArgoApplications represents the argo-rollout CRD state of the world that may or may not be present
	// in the client's cluster. For reasons why this is defined as unstructured see ArgoRollouts attribute.
	ArgoApplications []*unstructured.Unstructured `json:"ArgoApplications,omitempty"`
}

type SecretRef struct {
	Namespace string
	Name      string
}

type ConsulSnapshot struct {
	Endpoints map[string]consulwatch.Endpoints `json:",omitempty"`
}

type Delta struct {
	metav1.TypeMeta   `json:""`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	DeltaType         DeltaType `json:"deltaType"`
}

type DeltaType int

const (
	ObjectAdd DeltaType = iota
	ObjectUpdate
	ObjectDelete
)

// The APIDoc type is custom object built in the style of a Kubernetes resource (name, type, version)
// which holds a reference to a Kubernetes object from which an OpenAPI document was scrapped (Data field)
type APIDoc struct {
	*metav1.TypeMeta
	Metadata  *metav1.ObjectMeta      `json:"metadata,omitempty"`
	TargetRef *corev1.ObjectReference `json:"targetRef,omitempty"`
	Data      []byte                  `json:"data,omitempty"`
}

type IngressClass struct {
	types_net_v1.IngressClass
}

type Ingress struct {
	types_ext_v1beta1.Ingress
}

type Object interface {
	// runtime.Object gives the following methods:
	//
	//   GetObjectKind() k8s.io/apimachinery/pkg/runtime/schema.ObjectKind
	//   DeepCopyObject() k8s.io/apimachinery/pkg/runtime.Object
	runtime.Object

	// metav1.Object gives the following methods:
	//
	//   GetNamespace() string
	//   SetNamespace(namespace string)
	//   GetName() string
	//   SetName(name string)
	//   GetGenerateName() string
	//   SetGenerateName(name string)
	//   GetUID() k8s.io/apimachinery/pkg/types.UID
	//   SetUID(uid k8s.io/apimachinery/pkg/types.UID)
	//   GetResourceVersion() string
	//   SetResourceVersion(version string)
	//   GetGeneration() int64
	//   SetGeneration(generation int64)
	//   GetSelfLink() string
	//   SetSelfLink(selfLink string)
	//   GetCreationTimestamp() metav1.Time
	//   SetCreationTimestamp(timestamp metav1.Time)
	//   GetDeletionTimestamp() *metav1.Time
	//   SetDeletionTimestamp(timestamp *metav1.Time)
	//   GetDeletionGracePeriodSeconds() *int64
	//   SetDeletionGracePeriodSeconds(*int64)
	//   GetLabels() map[string]string
	//   SetLabels(labels map[string]string)
	//   GetAnnotations() map[string]string
	//   SetAnnotations(annotations map[string]string)
	//   GetFinalizers() []string
	//   SetFinalizers(finalizers []string)
	//   GetOwnerReferences() []metav1.OwnerReference
	//   SetOwnerReferences([]metav1.OwnerReference)
	//   GetClusterName() string
	//   SetClusterName(clusterName string)
	//   GetManagedFields() []metav1.ManagedFieldsEntry
	//   SetManagedFields(managedFields []metav1.ManagedFieldsEntry)
	metav1.Object
}
