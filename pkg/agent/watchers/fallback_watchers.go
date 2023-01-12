package watchers

import (
	"context"
	"sync"

	core "k8s.io/api/core/v1"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/k8sapi/pkg/k8sapi"
	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
)

type FallbackWatchers struct {
	cond            *sync.Cond
	serviceWatchers k8sapi.WatcherGroup[*core.Service]
	ingressWatchers ingressWatcher

	om ObjectModifier
}

func NewFallbackWatcher(ctx context.Context, namespaces []string, om ObjectModifier) *FallbackWatchers {
	coreClient := k8sapi.GetK8sInterface(ctx).CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// if there are no namespaces to watch, the watchers need to set their namespace to "",
	// which will set them to watch the whole cluster
	if len(namespaces) == 0 {
		namespaces = append(namespaces, "")
	}

	// TODO equals func to prevent over-broadcasting
	siWatcher := &FallbackWatchers{
		serviceWatchers: k8sapi.NewWatcherGroup[*core.Service](),
		ingressWatchers: getIngressWatcher(ctx, namespaces, cond, om),
		cond:            cond,
		om:              om,
	}

	for _, ns := range namespaces {
		_ = siWatcher.serviceWatchers.AddWatcher(
			k8sapi.NewWatcher[*core.Service]("services", coreClient, cond, k8sapi.WithNamespace[*core.Service](ns)))
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
	if w.om != nil {
		for _, svc := range snapshot.Kubernetes.Services {
			w.om(svc)
		}
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
