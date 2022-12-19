package watchers

import (
	"context"
	"fmt"
	"sync"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type EdgisarryWatchers struct {
	cond              *sync.Cond
	EndpointsWatchers k8sapi.WatcherGroup[*kates.Endpoints]
}

func NewEdgisarryWatchers(ctx context.Context, clientset *kubernetes.Clientset, namespaces []string) *EdgisarryWatchers {
	coreClient := clientset.CoreV1().RESTClient()
	ew := &EdgisarryWatchers{
		EndpointsWatchers: k8sapi.NewWatcherGroup[*kates.Endpoints](),
		cond:              &sync.Cond{L: &sync.Mutex{}},
	}

	// TODO equals func to prevent over-broadcasting
	for _, ns := range namespaces {
		for _, name := range []string{"emissary-ingress", "ambassador", "edge-stack"} {
			err := ew.EndpointsWatchers.AddWatcher(k8sapi.NewWatcher("endpoints", coreClient, ew.cond,
				k8sapi.WithNamespace[*kates.Endpoints](ns),
				k8sapi.WithFieldSelector[*kates.Endpoints](fmt.Sprintf("metadata.name=%s", name)),
				k8sapi.WithEquals(func(e1, e2 *v1.Endpoints) bool {
					// We only need the name.namespace of edgissary,
					// so don't broadcast any updates
					return true
				}),
			))
			// error here caused by duplicate ns, log err and continue
			if err != nil {
				dlog.Error(ctx, err)
			}
		}
	}

	return ew
}

func (e *EdgisarryWatchers) Subscribe(ctx context.Context) <-chan struct{} {
	return k8sapi.Subscribe(ctx, e.cond)
}
