package watchers

import (
	"context"
	"sync"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
	"k8s.io/client-go/kubernetes"
)

type FallbackWatchers struct {
	cond            *sync.Cond
	serviceWatchers k8sapi.WatcherGroup[*kates.Service]
	ingressWatchers ingressWatcher

	om ObjectModifier
}

func NewFallbackWatcher(ctx context.Context, clientset *kubernetes.Clientset, namespaces []string, om ObjectModifier) *FallbackWatchers {
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// TODO equals func to prevent over-broadcasting
	siWatcher := &FallbackWatchers{
		serviceWatchers: k8sapi.NewWatcherGroup[*kates.Service](),
		ingressWatchers: getIngressWatcher(ctx, clientset, namespaces, cond, om),
		cond:            cond,
		om:              om,
	}

	for _, ns := range namespaces {
		siWatcher.serviceWatchers.AddWatcher(k8sapi.NewWatcher("services", ns, coreClient, &kates.Service{}, cond, nil))
	}

	return siWatcher
}

func (w *FallbackWatchers) EnsureStarted(ctx context.Context) {
	w.serviceWatchers.EnsureStarted(ctx, nil)
	w.ingressWatchers.EnsureStarted(ctx, nil)
}

func (w *FallbackWatchers) Cancel() {
	w.serviceWatchers.Cancel()
	w.ingressWatchers.Cancel()
}

func (w *FallbackWatchers) LoadSnapshot(ctx context.Context, snapshot *snapshotTypes.Snapshot) {
	var err error
	if snapshot.Kubernetes.Services, err = w.serviceWatchers.List(ctx); err != nil {
		dlog.Errorf(ctx, "Unable to find services: %v", err)
	}
	dlog.Debugf(ctx, "Found %d services", len(snapshot.Kubernetes.Services))
	if ingresses, err := w.ingressWatchers.List(ctx); err != nil {
		dlog.Errorf(ctx, "Unable to find ingresses: %v", err)
	} else {
		snapshot.Kubernetes.Ingresses = []*snapshotTypes.Ingress{}
		for _, ing := range ingresses {
			if w.om != nil {
				w.om(ing)
			}
			snapshot.Kubernetes.Ingresses = append(snapshot.Kubernetes.Ingresses, &snapshotTypes.Ingress{Ingress: *ing})
		}
	}
	dlog.Debugf(ctx, "Found %d ingresses", len(snapshot.Kubernetes.Ingresses))
}

func (w *FallbackWatchers) Subscribe(ctx context.Context) <-chan struct{} {
	return k8sapi.Subscribe(ctx, w.cond)
}
