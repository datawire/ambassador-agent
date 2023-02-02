package watchers

import (
	"context"
	"sync"

	v1networking "k8s.io/api/networking/v1"

	"github.com/pkg/errors"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates/k8s_resource_types"
)

type ingressWatcher interface {
	List(ctx context.Context) ([]*k8s_resource_types.Ingress, error)
	EnsureStarted(ctx context.Context, cb func(bool))
	Cancel()
}

type networkWatcher struct {
	watcher k8sapi.WatcherGroup[*v1networking.Ingress]
	om      ObjectModifier
}

func (n *networkWatcher) EnsureStarted(ctx context.Context, cb func(bool)) {
	n.watcher.EnsureStarted(ctx, cb)
}

func (n *networkWatcher) Cancel() {
	n.watcher.Cancel()
}

func (n *networkWatcher) convertStatus(ing *v1networking.Ingress) extv1beta1.IngressStatus {
	lbis := make([]extv1beta1.IngressLoadBalancerIngress, len(ing.Status.LoadBalancer.Ingress))
	for i, lbi := range ing.Status.LoadBalancer.Ingress {
		ports := make([]extv1beta1.IngressPortStatus, len(lbi.Ports))
		for pi, port := range lbi.Ports {
			ports[pi] = extv1beta1.IngressPortStatus{
				Port:     port.Port,
				Error:    port.Error,
				Protocol: port.Protocol,
			}
		}
		lbis[i] = extv1beta1.IngressLoadBalancerIngress{
			IP:       lbi.IP,
			Hostname: lbi.Hostname,
			Ports:    ports,
		}
	}
	return extv1beta1.IngressStatus{
		LoadBalancer: extv1beta1.IngressLoadBalancerStatus{
			Ingress: lbis,
		},
	}
}

func (n *networkWatcher) convertIngressBackend(backend *v1networking.IngressBackend) *extv1beta1.IngressBackend {
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
		Resource:    backend.Resource,
	}
}

func (n *networkWatcher) convertSpec(ing *v1networking.Ingress) extv1beta1.IngressSpec {
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

	var defaultBackend *extv1beta1.IngressBackend
	if ing.Spec.DefaultBackend != nil {
		defaultBackend = n.convertIngressBackend(ing.Spec.DefaultBackend)
	}

	return extv1beta1.IngressSpec{
		IngressClassName: ing.Spec.IngressClassName,
		Backend:          defaultBackend,
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
		if n.om != nil {
			n.om(conv)
		}
		result = append(result, conv)
	}
	return result, nil
}

func isNetworkingAPIAvailable(ctx context.Context, clientset kubernetes.Interface, namespaces []string) bool {
	ns := ""
	if len(namespaces) > 0 {
		ns = namespaces[0]
	}
	_, err := clientset.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
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

func getIngressWatcher(ctx context.Context, namespaces []string, cond *sync.Cond, om ObjectModifier) ingressWatcher {
	k8sif := k8sapi.GetK8sInterface(ctx)
	if isNetworkingAPIAvailable(ctx, k8sif, namespaces) {
		netClient := k8sif.NetworkingV1().RESTClient()
		watcher := k8sapi.NewWatcherGroup[*v1networking.Ingress]()
		for _, ns := range namespaces {
			_ = watcher.AddWatcher(k8sapi.NewWatcher[*v1networking.Ingress]("ingresses", netClient, cond, k8sapi.WithNamespace[*v1networking.Ingress](ns)))
		}
		return &networkWatcher{watcher: watcher, om: om}
	}
	netClient := k8sif.ExtensionsV1beta1().RESTClient()
	watcher := k8sapi.NewWatcherGroup[*k8s_resource_types.Ingress]()
	for _, ns := range namespaces {
		_ = watcher.AddWatcher(k8sapi.NewWatcher[*k8s_resource_types.Ingress]("ingresses", netClient, cond, k8sapi.WithNamespace[*k8s_resource_types.Ingress](ns)))
	}
	return watcher
}
