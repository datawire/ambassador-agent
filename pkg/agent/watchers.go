package agent

import (
	"context"
	"sync"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
	"k8s.io/client-go/kubernetes"
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

type SIWatcher struct {
	cond            *sync.Cond
	serviceWatchers k8sapi.WatcherGroup[*kates.Service]
	ingressWatchers k8sapi.WatcherGroup[*snapshot.Ingress]
}

func NewSIWatcher(clientset *kubernetes.Clientset, namespaces []string) *SIWatcher {
	coreClient := clientset.CoreV1().RESTClient()
	netClient := clientset.NetworkingV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// TODO equals func to prevent over-broadcasting
	siWatcher := &SIWatcher{
		serviceWatchers: k8sapi.NewWatcherGroup[*kates.Service](),
		ingressWatchers: k8sapi.NewWatcherGroup[*snapshot.Ingress](),
		cond:            cond,
	}

	for _, ns := range namespaces {
		siWatcher.serviceWatchers.AddWatcher(k8sapi.NewWatcher("services", ns, coreClient, &kates.Service{}, cond, nil))
		siWatcher.ingressWatchers.AddWatcher(k8sapi.NewWatcher("ingresses", ns, netClient, &snapshot.Ingress{}, cond, nil))
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
