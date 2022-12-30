package agent

import (
	"context"
	"sync"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
)

type AmbassadorWatcher struct {
	cond            *sync.Cond
	endpointWatcher *k8sapi.Watcher[*kates.Endpoints]
}

func NewAmbassadorWatcher(ctx context.Context, ns string) *AmbassadorWatcher {
	coreClient := k8sapi.GetK8sInterface(ctx).CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	equalsFunc := func(obj1, obj2 *kates.Endpoints) bool {
		// we only care that the endpoint exists,
		// any updates with details are unnecessary
		return true
	}

	return &AmbassadorWatcher{
		cond: cond,
		endpointWatcher: k8sapi.NewWatcher[*kates.Endpoints](
			"endpoints", coreClient, cond, k8sapi.WithNamespace[*kates.Endpoints](ns), k8sapi.WithEquals(equalsFunc)),
	}
}

func (w *AmbassadorWatcher) EnsureStarted(ctx context.Context) error {
	return w.endpointWatcher.EnsureStarted(ctx, nil)
}
