package watchers

import (
	"context"
	"sync"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/k8sapi/pkg/k8sapi"
	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
)

type CoreWatchers struct {
	cond             *sync.Cond
	cmapsWatchers    k8sapi.WatcherGroup[*core.ConfigMap]
	deployWatchers   k8sapi.WatcherGroup[*apps.Deployment]
	podWatchers      k8sapi.WatcherGroup[*core.Pod]
	endpointWatchers k8sapi.WatcherGroup[*core.Endpoints]

	om ObjectModifier
}

func NewCoreWatchers(ctx context.Context, namespaces []string, om ObjectModifier) *CoreWatchers {
	k8sif := k8sapi.GetK8sInterface(ctx)
	appClient := k8sif.AppsV1().RESTClient()
	coreClient := k8sif.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	// if there are no namespaces to watch, create one watcher with namespace "",
	// which will watch the whole cluster
	if len(namespaces) == 0 {
		namespaces = append(namespaces, "")
	}

	coreWatchers := &CoreWatchers{
		cmapsWatchers:    k8sapi.NewWatcherGroup[*core.ConfigMap](),
		deployWatchers:   k8sapi.NewWatcherGroup[*apps.Deployment](),
		podWatchers:      k8sapi.NewWatcherGroup[*core.Pod](),
		endpointWatchers: k8sapi.NewWatcherGroup[*core.Endpoints](),
		cond:             cond,
		om:               om,
	}

	// TODO equals func to prevent over-broadcasting
	for _, ns := range namespaces {
		_ = coreWatchers.cmapsWatchers.AddWatcher(k8sapi.NewWatcher[*core.ConfigMap]("configmaps", coreClient, cond, k8sapi.WithNamespace[*core.ConfigMap](ns)))
		_ = coreWatchers.deployWatchers.AddWatcher(k8sapi.NewWatcher[*apps.Deployment]("deployments", appClient, cond, k8sapi.WithNamespace[*apps.Deployment](ns)))
		_ = coreWatchers.podWatchers.AddWatcher(k8sapi.NewWatcher[*core.Pod]("pods", coreClient, cond, k8sapi.WithNamespace[*core.Pod](ns)))
		_ = coreWatchers.endpointWatchers.AddWatcher(k8sapi.NewWatcher[*core.Endpoints]("endpoints", coreClient, cond, k8sapi.WithNamespace[*core.Endpoints](ns)))
	}

	return coreWatchers
}

func (w *CoreWatchers) loadPods(ctx context.Context) []*core.Pod {
	pods, err := w.podWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find pods: %v", err)
		return nil
	}

	fpods := make([]*core.Pod, 0, len(pods))
	for _, pod := range pods {
		if allowedNamespace(pod.GetNamespace()) && pod.Status.Phase != core.PodSucceeded {
			if w.om != nil {
				w.om(pod)
			}
			fpods = append(fpods, pod)
		}
	}

	return fpods
}

func (w *CoreWatchers) loadCmaps(ctx context.Context) []*core.ConfigMap {
	cmaps, err := w.cmapsWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find configmaps: %v", err)
		return nil
	}

	fcmaps := make([]*core.ConfigMap, 0, len(cmaps))
	for _, cmap := range cmaps {
		if allowedNamespace(cmap.GetNamespace()) {
			if w.om != nil {
				w.om(cmap)
			}
			fcmaps = append(fcmaps, cmap)
		}
	}

	return fcmaps
}

func (w *CoreWatchers) loadDeploys(ctx context.Context) []*apps.Deployment {
	deploys, err := w.deployWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find deployments: %v", err)
		return nil
	}

	fdeploys := make([]*apps.Deployment, 0, len(deploys))
	for _, deploy := range deploys {
		if allowedNamespace(deploy.GetNamespace()) {
			if w.om != nil {
				w.om(deploy)
			}
			fdeploys = append(fdeploys, deploy)
		}
	}

	return fdeploys
}

func (w *CoreWatchers) loadEndpoints(ctx context.Context) []*core.Endpoints {
	endpts, err := w.endpointWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find endpoints: %v", err)
		return nil
	}

	fendpts := make([]*core.Endpoints, 0, len(endpts))
	for _, endpt := range endpts {
		if allowedNamespace(endpt.GetNamespace()) {
			if w.om != nil {
				w.om(endpt)
			}
			fendpts = append(fendpts, endpt)
		}
	}

	return fendpts
}

// allowedNamespace will check if resources from the given namespace
// should be reported to Ambassador Cloud.
func allowedNamespace(namespace string) bool {
	return namespace != "kube-system"
}

func (w *CoreWatchers) LoadSnapshot(ctx context.Context, snapshot *snapshotTypes.Snapshot) {
	k8sSnap := snapshot.Kubernetes
	k8sSnap.Pods = w.loadPods(ctx)
	dlog.Debugf(ctx, "Found %d pods", len(k8sSnap.Pods))

	k8sSnap.ConfigMaps = w.loadCmaps(ctx)
	dlog.Debugf(ctx, "Found %d configMaps", len(k8sSnap.ConfigMaps))

	k8sSnap.Deployments = w.loadDeploys(ctx)
	dlog.Debugf(ctx, "Found %d Deployments", len(k8sSnap.Deployments))

	k8sSnap.Endpoints = w.loadEndpoints(ctx)
	dlog.Debugf(ctx, "Found %d Endpoints", len(k8sSnap.Endpoints))
}

func (w *CoreWatchers) Subscribe(ctx context.Context) <-chan struct{} {
	return k8sapi.Subscribe(ctx, w.cond)
}

func (w *CoreWatchers) EnsureStarted(ctx context.Context) {
	w.cmapsWatchers.EnsureStarted(ctx, nil)
	w.deployWatchers.EnsureStarted(ctx, nil)
	w.podWatchers.EnsureStarted(ctx, nil)
	w.endpointWatchers.EnsureStarted(ctx, nil)
}

func (w *CoreWatchers) Cancel() {
	w.cmapsWatchers.Cancel()
	w.deployWatchers.Cancel()
	w.podWatchers.Cancel()
	w.endpointWatchers.Cancel()
}
