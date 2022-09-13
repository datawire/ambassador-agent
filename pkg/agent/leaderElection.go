package agent

import (
	"context"
	"time"

	clientset "k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

type leaderElection struct {
	client               *clientset.Clientset
	OnStartedLeadingFunc func(context.Context)
	OnStoppedLeadingFunc func()
	OnNewLeaderFunc      func(newLeaderID string)
}

func (l leaderElection) getNewLock(lockname, podname, namespace string) *resourcelock.LeaseLock {
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockname,
			Namespace: namespace,
		},
		Client: l.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podname,
		},
	}
}

func (l leaderElection) runLeaderElection(lock *resourcelock.LeaseLock, ctx context.Context, id string) {
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: l.OnStartedLeadingFunc,
			OnStoppedLeading: l.OnStoppedLeadingFunc,
			OnNewLeader:      l.OnNewLeaderFunc,
		},
	})
}
