package agent

import (
	"context"
	"sync"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"k8s.io/client-go/kubernetes"
)

type Watchers struct {
	cond            *sync.Cond
	mapsWatcher     *k8sapi.Watcher[*kates.ConfigMap]
	deployWatcher   *k8sapi.Watcher[*kates.Deployment]
	podWatcher      *k8sapi.Watcher[*kates.Pod]
	endpointWatcher *k8sapi.Watcher[*kates.Endpoints]
}

func NewWatchers(clientset *kubernetes.Clientset) *Watchers {
	appClient := clientset.AppsV1().RESTClient()
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// TODO scoped agent logic
	watchedNs := "" // empty string watches all ns

	// TODO equals func
	return &Watchers{
		mapsWatcher:     k8sapi.NewWatcher("configmaps", watchedNs, coreClient, &kates.ConfigMap{}, cond, nil),
		deployWatcher:   k8sapi.NewWatcher("deployments", watchedNs, appClient, &kates.Deployment{}, cond, nil),
		podWatcher:      k8sapi.NewWatcher("pods", watchedNs, coreClient, &kates.Pod{}, cond, nil),
		endpointWatcher: k8sapi.NewWatcher("endpoints", watchedNs, coreClient, &kates.Endpoints{}, cond, nil),
		cond:            cond,
	}
}

func (w *Watchers) EnsureStarted(ctx context.Context) {
	w.mapsWatcher.EnsureStarted(ctx, nil)
	w.deployWatcher.EnsureStarted(ctx, nil)
	w.podWatcher.EnsureStarted(ctx, nil)
	w.endpointWatcher.EnsureStarted(ctx, nil)
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

	return &AmbassadorWatcher{
		cond:            cond,
		endpointWatcher: k8sapi.NewWatcher("endpoints", ns, coreClient, &kates.Endpoints{}, cond, nil),
	}
}

func (w *AmbassadorWatcher) EnsureStarted(ctx context.Context) {
	w.endpointWatcher.EnsureStarted(ctx, nil)
}

type SIWatcher struct {
	cond           *sync.Cond
	serviceWatcher *k8sapi.Watcher[*kates.Service]
}

func NewSIWatcher(clientset *kubernetes.Clientset) *SIWatcher {
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// TODO scoped agent logic
	watchedNs := "" // empty string watches all ns

	// TODO equals func
	return &SIWatcher{
		cond:           cond,
		serviceWatcher: k8sapi.NewWatcher("service", watchedNs, coreClient, &kates.Service{}, cond, nil),
	}
}

func (w *SIWatcher) EnsureStarted(ctx context.Context) {
	w.serviceWatcher.EnsureStarted(ctx, nil)
}

func (w *SIWatcher) Cancel() {
	w.serviceWatcher.Cancel()
}
