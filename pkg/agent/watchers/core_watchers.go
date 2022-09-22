package watchers

import (
	"context"
	"sync"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"

	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type CoreWatchers struct {
	cond             *sync.Cond
	cmapsWatchers    k8sapi.WatcherGroup[*kates.ConfigMap]
	deployWatchers   k8sapi.WatcherGroup[*kates.Deployment]
	podWatchers      k8sapi.WatcherGroup[*kates.Pod]
	endpointWatchers k8sapi.WatcherGroup[*kates.Endpoints]
}

func NewCoreWatchers(clientset *kubernetes.Clientset, namespaces []string) *CoreWatchers {
	appClient := clientset.AppsV1().RESTClient()
	coreClient := clientset.CoreV1().RESTClient()

	cond := &sync.Cond{
		L: &sync.Mutex{},
	}

	coreWatchers := &CoreWatchers{
		cmapsWatchers:    k8sapi.NewWatcherGroup[*kates.ConfigMap](),
		deployWatchers:   k8sapi.NewWatcherGroup[*kates.Deployment](),
		podWatchers:      k8sapi.NewWatcherGroup[*kates.Pod](),
		endpointWatchers: k8sapi.NewWatcherGroup[*kates.Endpoints](),
		cond:             cond,
	}

	// TODO equals func to prevent over-broadcasting
	for _, ns := range namespaces {
		coreWatchers.cmapsWatchers.AddWatcher(k8sapi.NewWatcher("configmaps", ns, coreClient, &kates.ConfigMap{}, cond, nil))
		coreWatchers.deployWatchers.AddWatcher(k8sapi.NewWatcher("deployments", ns, appClient, &kates.Deployment{}, cond, nil))
		coreWatchers.podWatchers.AddWatcher(k8sapi.NewWatcher("pods", ns, coreClient, &kates.Pod{}, cond, nil))
		coreWatchers.endpointWatchers.AddWatcher(k8sapi.NewWatcher("endpoints", ns, coreClient, &kates.Endpoints{}, cond, nil))
	}

	return coreWatchers
}

func labelMatching(pod *kates.Pod, svcs []*kates.Service) bool {
	matchSvc := func(pod *kates.Pod, svc *kates.Service) bool {
		if pod.Namespace != svc.Namespace {
			return false
		}

		for k, v := range svc.Spec.Selector {
			if pod.Labels[k] != v {
				return false
			}
		}
		return true
	}

	for _, svc := range svcs {
		if matchSvc(pod, svc) {
			return true
		}
	}

	return false
}

func (w *CoreWatchers) loadPods(ctx context.Context, svcs []*kates.Service) []*kates.Pod {
	pods, err := w.podWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find pods: %v", err)
		return nil
	}

	fpods := make([]*kates.Pod, 0)
	for _, pod := range pods {
		if allowedNamespace(pod.GetNamespace()) && pod.Status.Phase != v1.PodSucceeded && labelMatching(pod, svcs) {
			fpods = append(fpods, pod)
		}
	}

	return fpods
}

func (w *CoreWatchers) loadCmaps(ctx context.Context) []*kates.ConfigMap {
	cmaps, err := w.cmapsWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find configmaps: %v", err)
		return nil
	}

	fcmaps := make([]*kates.ConfigMap, 0)
	for _, cmap := range cmaps {
		if allowedNamespace(cmap.GetNamespace()) {
			fcmaps = append(fcmaps, cmap)
		}
	}

	return fcmaps
}

func (w *CoreWatchers) loadDeploys(ctx context.Context) []*kates.Deployment {
	deploys, err := w.deployWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find deployments: %v", err)
		return nil
	}

	fdeploys := make([]*kates.Deployment, 0)
	for _, deploy := range deploys {
		if allowedNamespace(deploy.GetNamespace()) {
			fdeploys = append(fdeploys, deploy)
		}
	}

	return fdeploys
}

func (w *CoreWatchers) loadEndpoints(ctx context.Context) []*kates.Endpoints {
	endpts, err := w.endpointWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find endpoints: %v", err)
		return nil
	}

	fendpts := make([]*kates.Endpoints, 0)
	for _, endpt := range endpts {
		if allowedNamespace(endpt.GetNamespace()) {
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
	snapshot.Kubernetes.Pods = w.loadPods(ctx, snapshot.Kubernetes.Services)
	dlog.Debugf(ctx, "Found %d pods", len(snapshot.Kubernetes.Pods))

	snapshot.Kubernetes.ConfigMaps = w.loadCmaps(ctx)
	dlog.Debugf(ctx, "Found %d configMaps", len(snapshot.Kubernetes.ConfigMaps))

	snapshot.Kubernetes.Deployments = w.loadDeploys(ctx)
	dlog.Debugf(ctx, "Found %d Deployments", len(snapshot.Kubernetes.Deployments))

	snapshot.Kubernetes.Endpoints = w.loadEndpoints(ctx)
	dlog.Debugf(ctx, "Found %d Endpoints", len(snapshot.Kubernetes.Endpoints))
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
