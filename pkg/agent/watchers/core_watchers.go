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
	cmapsWatchers    k8sapi.WatcherGroup[*kates.ConfigMap]
	deployWatchers   k8sapi.WatcherGroup[*kates.Deployment]
	podWatchers      k8sapi.WatcherGroup[*kates.Pod]
	endpointWatchers k8sapi.WatcherGroup[*kates.Endpoints]

	om ObjectModifier
}

func NewCoreWatchers(clientset *kubernetes.Clientset, namespaces []string, om ObjectModifier) *CoreWatchers {
	appClient := clientset.AppsV1().RESTClient()
	coreClient := clientset.CoreV1().RESTClient()

	coreWatchers := &CoreWatchers{
		cmapsWatchers:    k8sapi.NewWatcherGroup[*kates.ConfigMap](),
		deployWatchers:   k8sapi.NewWatcherGroup[*kates.Deployment](),
		podWatchers:      k8sapi.NewWatcherGroup[*kates.Pod](),
		endpointWatchers: k8sapi.NewWatcherGroup[*kates.Endpoints](),
		om:               om,
	}

	// TODO equals func to prevent over-broadcasting
	for _, ns := range namespaces {
		coreWatchers.cmapsWatchers.AddWatcher(k8sapi.NewWatcher("configmaps", coreClient, &sync.Cond{L: &sync.Mutex{}},
			k8sapi.WithNamespace[*kates.ConfigMap](ns),
		))
		coreWatchers.deployWatchers.AddWatcher(k8sapi.NewWatcher("deployments", appClient, &sync.Cond{L: &sync.Mutex{}},
			k8sapi.WithNamespace[*kates.Deployment](ns),
		))
		coreWatchers.podWatchers.AddWatcher(k8sapi.NewWatcher("pods", coreClient, &sync.Cond{L: &sync.Mutex{}},
			k8sapi.WithNamespace[*kates.Pod](ns),
		))
		coreWatchers.endpointWatchers.AddWatcher(k8sapi.NewWatcher("endpoints", coreClient, &sync.Cond{L: &sync.Mutex{}},
			k8sapi.WithNamespace[*kates.Endpoints](ns),
		))
	}

	return coreWatchers
}

func (w *CoreWatchers) loadPods(ctx context.Context, svcs []*kates.Service) []*kates.Pod {
	pods, err := w.podWatchers.List(ctx)
	if err != nil {
		dlog.Errorf(ctx, "Unable to find pods: %v", err)
		return nil
	}

	fpods := make([]*kates.Pod, 0)
	for _, pod := range pods {
		if allowedNamespace(pod.GetNamespace()) && pod.Status.Phase != v1.PodSucceeded {
			if w.om != nil {
				w.om(pod)
			}
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
			if w.om != nil {
				w.om(cmap)
			}
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
			if w.om != nil {
				w.om(deploy)
			}
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
	snapshot.Kubernetes.Pods = w.loadPods(ctx, snapshot.Kubernetes.Services)
	dlog.Debugf(ctx, "Found %d pods", len(snapshot.Kubernetes.Pods))

	snapshot.Kubernetes.ConfigMaps = w.loadCmaps(ctx)
	dlog.Debugf(ctx, "Found %d configMaps", len(snapshot.Kubernetes.ConfigMaps))

	snapshot.Kubernetes.Deployments = w.loadDeploys(ctx)
	dlog.Debugf(ctx, "Found %d Deployments", len(snapshot.Kubernetes.Deployments))

	snapshot.Kubernetes.Endpoints = w.loadEndpoints(ctx)
	dlog.Debugf(ctx, "Found %d Endpoints", len(snapshot.Kubernetes.Endpoints))
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
