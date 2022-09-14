package agent

import (
	"context"
	"sync"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type Watchers struct {
	cond            *sync.Cond
	mapsWatcher     *k8sapi.Watcher[*kates.ConfigMap]
	deployWatcher   *k8sapi.Watcher[*kates.Deployment]
	podWatcher      *k8sapi.Watcher[*kates.Pod]
	endpointWatcher *k8sapi.Watcher[*kates.Endpoints]
}

func NewWatchers(appClient rest.Interface, coreClient rest.Interface) *Watchers {
	// This function compares the recieved object with the cached object
	// and decides whether an update should be pushed
	compareFunc := func(o1, o2 runtime.Object) bool {
		// TODO impl
		return true
	}

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	watchedNs := "" // empty string watches all ns

	return &Watchers{
		mapsWatcher:     k8sapi.NewWatcher("configmaps", watchedNs, coreClient, &kates.ConfigMap{}, cond, compareFunc),
		deployWatcher:   k8sapi.NewWatcher("deployments", watchedNs, appClient, &kates.Deployment{}, cond, compareFunc),
		podWatcher:      k8sapi.NewWatcher("pods", watchedNs, coreClient, &kates.Pod{}, cond, compareFunc),
		endpointWatcher: k8sapi.NewWatcher("endpoints", watchedNs, coreClient, &kates.Endpoints{}, cond, compareFunc),
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

func NewConfigWatchers(client rest.Interface, watchedNs string) *ConfigWatchers {
	// This function compares the recieved object with the cached object
	// and decides whether an update should be pushed
	compareFunc := func(o1, o2 runtime.Object) bool {
		// TODO impl
		return true
	}

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	return &ConfigWatchers{
		mapsWatcher:   k8sapi.NewWatcher("configmaps", watchedNs, client, &kates.ConfigMap{}, cond, compareFunc),
		secretWatcher: k8sapi.NewWatcher("secrets", watchedNs, client, &kates.Secret{}, cond, compareFunc),
		cond:          cond,
	}
}

func (w *ConfigWatchers) EnsureStarted(ctx context.Context) {
	w.mapsWatcher.EnsureStarted(ctx, nil)
	w.secretWatcher.EnsureStarted(ctx, nil)
}
