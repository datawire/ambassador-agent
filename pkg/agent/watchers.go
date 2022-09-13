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

func NewWatchers(client rest.Interface) *Watchers {
	// This function compares the recieved object with the cached object
	// and decides whether an update should be pushed
	compareFunc := func(o1, o2 runtime.Object) bool {
		// TODO impl
		return true
	}

	cond := &sync.Cond{
		L: &sync.RWMutex{},
	}

	watchedNs := ""

	return &Watchers{
		mapsWatcher:     k8sapi.NewWatcher("configmap", watchedNs, client, &kates.ConfigMap{}, cond, compareFunc),
		deployWatcher:   k8sapi.NewWatcher("deploy", watchedNs, client, &kates.Deployment{}, cond, compareFunc),
		podWatcher:      k8sapi.NewWatcher("pod", watchedNs, client, &kates.Pod{}, cond, compareFunc),
		endpointWatcher: k8sapi.NewWatcher("endpoints", watchedNs, client, &kates.Endpoints{}, cond, compareFunc),
		cond:            cond,
	}
}

func (w *Watchers) EnsureStarted(ctx context.Context) {
	w.mapsWatcher.EnsureStarted(ctx, nil)
	w.deployWatcher.EnsureStarted(ctx, nil)
	w.podWatcher.EnsureStarted(ctx, nil)
	w.endpointWatcher.EnsureStarted(ctx, nil)
}

type APIWatchers struct {
	cond          *sync.Cond
	mapsWatcher   *k8sapi.Watcher[*kates.ConfigMap]
	secretWatcher *k8sapi.Watcher[*kates.Secret]
}

func NewAPIWatchers(client rest.Interface) *APIWatchers {
	// This function compares the recieved object with the cached object
	// and decides whether an update should be pushed
	compareFunc := func(o1, o2 runtime.Object) bool {
		// TODO impl
		return true
	}

	cond := &sync.Cond{
		L: &sync.RWMutex{},
	}

	watchedNs := ""

	return &APIWatchers{
		mapsWatcher:   k8sapi.NewWatcher("configmap", watchedNs, client, &kates.ConfigMap{}, cond, compareFunc),
		secretWatcher: k8sapi.NewWatcher("secret", watchedNs, client, &kates.Secret{}, cond, compareFunc),
		cond:          cond,
	}
}

func (w *APIWatchers) EnsureStarted(ctx context.Context) {
	w.mapsWatcher.EnsureStarted(ctx, nil)
	w.secretWatcher.EnsureStarted(ctx, nil)
}
