package agent

import (
	"context"
	"sync"

	"k8s.io/client-go/kubernetes"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
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
		mapsWatcher: k8sapi.NewWatcher("configmaps", watchedNs, coreClient, &kates.ConfigMap{}, cond, func(o1, o2 *kates.ConfigMap) bool {
			// TODO equals func to prevent over-broadcasting
			return false
		}),
		secretWatcher: k8sapi.NewWatcher("secrets", watchedNs, coreClient, &kates.Secret{}, cond, func(o1, o2 *kates.Secret) bool {
			// TODO equals func to prevent over-broadcasting
			return false
		}),
		cond: cond,
	}
}

func (w *ConfigWatchers) EnsureStarted(ctx context.Context) {
	w.mapsWatcher.EnsureStarted(ctx, nil)
	w.secretWatcher.EnsureStarted(ctx, nil)
}
