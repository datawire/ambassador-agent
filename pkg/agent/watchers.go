package agent

import (
	"context"
	"sync"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"github.com/emissary-ingress/emissary/v3/pkg/kates/k8s_resource_types"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"k8s.io/kubernetes/pkg/apis/networking"
)

type CoreWatchers struct {
	cond             *sync.Cond
	mapsWatchers     k8sapi.WatcherGroup[*kates.ConfigMap]
	deployWatchers   k8sapi.WatcherGroup[*kates.Deployment]
	podWatchers      k8sapi.WatcherGroup[*kates.Pod]
	endpointWatchers k8sapi.WatcherGroup[*kates.Endpoints]
}

func NewCoreWatchers(clientset *kubernetes.Clientset, namespaces []string) *CoreWatchers {
	appClient := clientset.AppsV1().RESTClient()
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	coreWatchers := &CoreWatchers{
		mapsWatchers:     k8sapi.NewWatcherGroup[*kates.ConfigMap](),
		deployWatchers:   k8sapi.NewWatcherGroup[*kates.Deployment](),
		podWatchers:      k8sapi.NewWatcherGroup[*kates.Pod](),
		endpointWatchers: k8sapi.NewWatcherGroup[*kates.Endpoints](),
		cond:             cond,
	}

	// TODO equals func to prevent over-broadcasting
	for _, ns := range namespaces {
		coreWatchers.mapsWatchers.AddWatcher(k8sapi.NewWatcher("configmaps", ns, coreClient, &kates.ConfigMap{}, cond, nil))
		coreWatchers.deployWatchers.AddWatcher(k8sapi.NewWatcher("deployments", ns, appClient, &kates.Deployment{}, cond, nil))
		coreWatchers.podWatchers.AddWatcher(k8sapi.NewWatcher("pods", ns, coreClient, &kates.Pod{}, cond, nil))
		coreWatchers.endpointWatchers.AddWatcher(k8sapi.NewWatcher("endpoints", ns, coreClient, &kates.Endpoints{}, cond, nil))
	}

	return coreWatchers
}

func (w *CoreWatchers) EnsureStarted(ctx context.Context) {
	w.mapsWatchers.EnsureStarted(ctx, nil)
	w.deployWatchers.EnsureStarted(ctx, nil)
	w.podWatchers.EnsureStarted(ctx, nil)
	w.endpointWatchers.EnsureStarted(ctx, nil)
}

type ConfigWatchers struct {
	cond          *sync.Cond
	mapsWatcher   *k8sapi.Watcher[*kates.ConfigMap]
	secretWatcher *k8sapi.Watcher[*kates.Secret]
}

func NewConfigWatchers(clientset *kubernetes.Clientset, watchedNs string) *ConfigWatchers {
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// TODO equals func to prevent over-broadcasting
	return &ConfigWatchers{
		mapsWatcher:   k8sapi.NewWatcher("configmaps", watchedNs, coreClient, &kates.ConfigMap{}, cond, nil),
		secretWatcher: k8sapi.NewWatcher("secrets", watchedNs, coreClient, &kates.Secret{}, cond, nil),
		cond:          cond,
	}
}

func (w *ConfigWatchers) EnsureStarted(ctx context.Context) {
	w.mapsWatcher.EnsureStarted(ctx, nil)
	w.secretWatcher.EnsureStarted(ctx, nil)
}

type AmbassadorWatcher struct {
	cond            *sync.Cond
	endpointWatcher *k8sapi.Watcher[*kates.Endpoints]
}

func NewAmbassadorWatcher(clientset *kubernetes.Clientset, ns string) *AmbassadorWatcher {
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// TODO equals func to prevent over-broadcasting
	return &AmbassadorWatcher{
		cond:            cond,
		endpointWatcher: k8sapi.NewWatcher("endpoints", ns, coreClient, &kates.Endpoints{}, cond, nil),
	}
}

func (w *AmbassadorWatcher) EnsureStarted(ctx context.Context) {
	w.endpointWatcher.EnsureStarted(ctx, nil)
}

type ingressWatcher interface {
	List(ctx context.Context) ([]*k8s_resource_types.Ingress, error)
	EnsureStarted(ctx context.Context, cb func(bool))
	Cancel()
}

type SIWatcher struct {
	cond            *sync.Cond
	serviceWatchers k8sapi.WatcherGroup[*kates.Service]
	ingressWatchers ingressWatcher
}

type networkWatcher struct {
	watcher k8sapi.WatcherGroup[*networking.Ingress]
}

func (n *networkWatcher) EnsureStarted(ctx context.Context, cb func(bool)) {
	n.watcher.EnsureStarted(ctx, cb)
}

func (n *networkWatcher) Cancel() {
	n.watcher.Cancel()
}

func (n *networkWatcher) convertStatus(ing *networking.Ingress) extv1beta1.IngressStatus {
	lbis := []corev1.LoadBalancerIngress{}
	for _, lbi := range ing.Status.LoadBalancer.Ingress {
		ports := []corev1.PortStatus{}
		for _, port := range lbi.Ports {
			ports = append(ports, corev1.PortStatus{
				Port:     port.Port,
				Error:    port.Error,
				Protocol: corev1.Protocol(port.Protocol),
			})
		}
		lbis = append(lbis, corev1.LoadBalancerIngress{
			IP:       lbi.IP,
			Hostname: lbi.Hostname,
			Ports:    ports,
		})
	}
	return extv1beta1.IngressStatus{
		LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: lbis,
		},
	}
}

func (n *networkWatcher) convertIngressBackend(backend *networking.IngressBackend) *extv1beta1.IngressBackend {
	var servicePort intstr.IntOrString
	pt := backend.Service.Port
	if pt.Name != "" {
		servicePort = intstr.FromString(pt.Name)
	} else {
		servicePort = intstr.FromInt(int(pt.Number))
	}

	return &extv1beta1.IngressBackend{
		ServiceName: backend.Service.Name,
		ServicePort: servicePort,
		Resource:    (*corev1.TypedLocalObjectReference)(backend.Resource),
	}
}

func (n *networkWatcher) convertSpec(ing *networking.Ingress) extv1beta1.IngressSpec {
	tlss := []extv1beta1.IngressTLS{}
	for _, tls := range ing.Spec.TLS {
		tlss = append(tlss, extv1beta1.IngressTLS{
			Hosts:      tls.Hosts,
			SecretName: tls.SecretName,
		})
	}
	rules := []extv1beta1.IngressRule{}
	for _, rule := range ing.Spec.Rules {
		paths := []extv1beta1.HTTPIngressPath{}
		for _, path := range rule.HTTP.Paths {
			paths = append(paths, extv1beta1.HTTPIngressPath{
				Path:     path.Path,
				PathType: (*extv1beta1.PathType)(path.PathType),
				Backend:  *n.convertIngressBackend(&path.Backend),
			})
		}
		rules = append(rules, extv1beta1.IngressRule{
			Host: rule.Host,
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		})
	}
	return extv1beta1.IngressSpec{
		IngressClassName: ing.Spec.IngressClassName,
		Backend:          n.convertIngressBackend(ing.Spec.DefaultBackend),
		TLS:              tlss,
		Rules:            rules,
	}
}

func (n *networkWatcher) List(ctx context.Context) ([]*k8s_resource_types.Ingress, error) {
	ingresses, err := n.watcher.List(ctx)
	if err != nil {
		return nil, err
	}
	result := []*k8s_resource_types.Ingress{}
	for _, ing := range ingresses {
		conv := &k8s_resource_types.Ingress{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Ingress",
				APIVersion: "extensions/v1beta1",
			},
			ObjectMeta: ing.ObjectMeta,
			Status:     n.convertStatus(ing),
			Spec:       n.convertSpec(ing),
		}
		result = append(result, conv)
	}
	return result, nil
}

func isNetworkingAPIAvailable(ctx context.Context, clientset *kubernetes.Clientset, namespace string) bool {
	if namespace == "" {
		namespace = "default"
	}
	_, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	// The truth of the matter is, if we get an error other than NotFound that means the user is trying to use
	// the networking API but will not succeed; in that case, just let the watcher be created as normal, then have
	// its own error handling take care of the issue.
	if err != nil {
		se := &apierrors.StatusError{}
		if errors.As(err, &se) {
			if se.Status().Reason == metav1.StatusReasonNotFound {
				return false
			}
		}
	}
	return true
}

func getIngressWatcher(ctx context.Context, clientset *kubernetes.Clientset, namespaces []string, cond *sync.Cond) ingressWatcher {
	if isNetworkingAPIAvailable(ctx, clientset, namespaces[0]) {
		netClient := clientset.NetworkingV1().RESTClient()
		watcher := k8sapi.NewWatcherGroup[*networking.Ingress]()
		for _, ns := range namespaces {
			watcher.AddWatcher(k8sapi.NewWatcher("ingresses", ns, netClient, &networking.Ingress{}, cond, nil))
		}
		return &networkWatcher{watcher: watcher}
	}
	netClient := clientset.ExtensionsV1beta1().RESTClient()
	watcher := k8sapi.NewWatcherGroup[*k8s_resource_types.Ingress]()
	for _, ns := range namespaces {
		watcher.AddWatcher(k8sapi.NewWatcher("ingresses", ns, netClient, &k8s_resource_types.Ingress{}, cond, nil))
	}
	return watcher
}

func NewSIWatcher(ctx context.Context, clientset *kubernetes.Clientset, namespaces []string) *SIWatcher {
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// TODO equals func to prevent over-broadcasting
	siWatcher := &SIWatcher{
		serviceWatchers: k8sapi.NewWatcherGroup[*kates.Service](),
		ingressWatchers: getIngressWatcher(ctx, clientset, namespaces, cond),
		cond:            cond,
	}

	for _, ns := range namespaces {
		siWatcher.serviceWatchers.AddWatcher(k8sapi.NewWatcher("services", ns, coreClient, &kates.Service{}, cond, nil))
	}

	return siWatcher
}

func (w *SIWatcher) EnsureStarted(ctx context.Context) {
	w.serviceWatchers.EnsureStarted(ctx, nil)
	w.ingressWatchers.EnsureStarted(ctx, nil)
}

func (w *SIWatcher) Cancel() {
	w.serviceWatchers.Cancel()
	w.ingressWatchers.Cancel()
}
