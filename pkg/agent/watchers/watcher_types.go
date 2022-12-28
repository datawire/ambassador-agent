package watchers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
)

type ObjectModifier func(obj runtime.Object)

//go:generate mockgen -destination=mocks/serviceeventsservice_mock.go . SnapshotWatcher
type SnapshotWatcher interface {
	LoadSnapshot(ctx context.Context, snapshot *snapshotTypes.Snapshot)
	Subscribe(ctx context.Context) <-chan struct{}
	EnsureStarted(ctx context.Context)
	Cancel()
}
