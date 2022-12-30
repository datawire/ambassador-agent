package agent

import (
	"context"
	"sync"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
)

type ConfigWatchers struct {
	cond          *sync.Cond
	mapsWatcher   *k8sapi.Watcher[*kates.ConfigMap]
	secretWatcher *k8sapi.Watcher[*kates.Secret]
}

func NewConfigWatchers(ctx context.Context, watchedNs string) *ConfigWatchers {
	coreClient := k8sapi.GetK8sInterface(ctx).CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	return &ConfigWatchers{
		mapsWatcher: k8sapi.NewWatcher[*kates.ConfigMap]("configmaps", coreClient, cond,
			k8sapi.WithNamespace[*kates.ConfigMap](watchedNs),
			k8sapi.WithEquals(func(o1, o2 *kates.ConfigMap) bool {
				// TODO equals func to prevent over-broadcasting
				return false
			})),
		secretWatcher: k8sapi.NewWatcher[*kates.Secret]("secrets", coreClient, cond,
			k8sapi.WithNamespace[*kates.Secret](watchedNs),
			k8sapi.WithEquals(func(o1, o2 *kates.Secret) bool {
				// TODO equals func to prevent over-broadcasting
				return false
			})),
		cond: cond,
	}
}

func (w *ConfigWatchers) EnsureStarted(ctx context.Context) error {
	if err := w.mapsWatcher.EnsureStarted(ctx, nil); err != nil {
		return err
	}
	return w.secretWatcher.EnsureStarted(ctx, nil)
}
