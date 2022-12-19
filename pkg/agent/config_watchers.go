package agent

import (
	"context"
	"sync"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"k8s.io/client-go/kubernetes"
)

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
		// TODO equals func to prevent over-broadcasting
		mapsWatcher: k8sapi.NewWatcher("configmaps", coreClient, cond,
			k8sapi.WithNamespace[*kates.ConfigMap](watchedNs),
		),
		secretWatcher: k8sapi.NewWatcher("secrets", coreClient, cond,
			k8sapi.WithNamespace[*kates.Secret](watchedNs),
		),
		cond: cond,
	}
}

func (w *ConfigWatchers) EnsureStarted(ctx context.Context) {
	w.mapsWatcher.EnsureStarted(ctx, nil)
	w.secretWatcher.EnsureStarted(ctx, nil)
}
