package agent

import (
	"context"
	"sync"

	"k8s.io/client-go/kubernetes"

	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
)

type AmbassadorWatcher struct {
	cond            *sync.Cond
	endpointWatcher *k8sapi.Watcher[*kates.Endpoints]
}

func NewAmbassadorWatcher(clientset *kubernetes.Clientset, ns string) *AmbassadorWatcher {
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	equalsFunc := func(obj1, obj2 *kates.Endpoints) bool {
		// we only care that the endpoint exists,
		// any updates with details are unnecessary
		return true
	}

	return &AmbassadorWatcher{
		cond:            cond,
		endpointWatcher: k8sapi.NewWatcher("endpoints", ns, coreClient, &kates.Endpoints{}, cond, equalsFunc),
	}
}

func (w *AmbassadorWatcher) EnsureStarted(ctx context.Context) {
	w.endpointWatcher.EnsureStarted(ctx, nil)
}
