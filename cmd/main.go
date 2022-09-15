package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/datawire/ambassador-agent/pkg/agent"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/datawire/dlib/dgroup"
	"github.com/datawire/dlib/dlog"
)

// internal k8s service
const (
	AdminDiagnosticsPort     = 8877
	DefaultSnapshotURLFmt    = "http://ambassador-admin:%d/snapshot-external"
	DefaultDiagnosticsURLFmt = "http://ambassador-admin:%d/ambassador/v0/diag/?json=true"

	ExternalSnapshotPort = 8005

	leaseLockName = "ambassador-agent-lease-lock"
)

func main() {
	ctx := context.Background()

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		dlog.Error(ctx, err.Error())
		os.Exit(1)
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)
	agentNamespace := getEnvWithDefault("AGENT_NAMESPACE", "ambassador")
	ambAgent := agent.NewAgent(nil, agent.NewArgoRolloutsGetter, agent.NewSecretsGetter, clientset, agentNamespace)

	snapshotURL := getEnvWithDefault("AES_SNAPSHOT_URL", fmt.Sprintf(DefaultSnapshotURLFmt, ExternalSnapshotPort))
	diagnosticsURL := getEnvWithDefault("AES_DIAGNOSTICS_URL", fmt.Sprintf(DefaultDiagnosticsURLFmt, AdminDiagnosticsPort))
	reportDiagnostics := os.Getenv("AES_REPORT_DIAGNOSTICS_TO_CLOUD")
	if reportDiagnostics == "true" {
		ambAgent.SetReportDiagnosticsAllowed(true)
	}

	metricsListener, err := net.Listen("tcp", ":8080")
	if err != nil {
		dlog.Error(ctx, err.Error())
		os.Exit(1)
	}
	dlog.Info(ctx, "metrics service listening on :8080")

	grp := dgroup.NewGroup(ctx, dgroup.GroupConfig{})
	grp.Go("metrics-server", func(ctx context.Context) error {
		metricsServer := agent.NewMetricsServer(ambAgent.MetricsRelayHandler)
		return metricsServer.Serve(ctx, metricsListener)
	})

	// use a Go context so we can tell the leaderelection code when we
	// want to step down
	leaseCtx, leaseCancel := context.WithCancel(ctx)
	defer leaseCancel()

	// listen for interrupts or the Linux SIGTERM signal and cancel
	// our context, which the leader election code will observe and
	// step down
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		dlog.Info(ctx, "Received termination, signaling shutdown")
		leaseCancel()
		os.Exit(0)
	}()

	// each call to the leaselock should have a unique id
	id := uuid.New().String()
	dlog.Infof(ctx, "Will lease with id %s", id)

	// we use the Lease lock type since edits to Leases are less common
	// and fewer objects in the cluster watch "all Leases".
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: agentNamespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	// use a go context to kill watchers OnStoppedLeading
	var (
		watchCtx    context.Context
		watchCancel context.CancelFunc
		i           int
	)
	run := func(ctx context.Context) {
		i += 1
		grp.Go(fmt.Sprintf("watch-%v", i), func(grpCtx context.Context) error {
			watchCtx, watchCancel = context.WithCancel(ctx)
			defer watchCancel()
			go func() {
				select {
				case <-watchCtx.Done():
				case <-grpCtx.Done():
					watchCancel()
				}
			}()
			return ambAgent.Watch(watchCtx, snapshotURL, diagnosticsURL)
		})
	}

	// start the leader election code loop
	leaderelection.RunOrDie(leaseCtx, leaderelection.LeaderElectionConfig{
		Lock: lock,
		// IMPORTANT: you MUST ensure that any code you have that
		// is protected by the lease must terminate **before**
		// you call cancel. Otherwise, you could have a background
		// loop still running and another process could
		// get elected before your background loop finished, violating
		// the stated goal of the lease.
		ReleaseOnCancel: true,
		LeaseDuration:   60 * time.Second,
		RenewDeadline:   40 * time.Second,
		RetryPeriod:     8 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// we're notified when we start - this is where you would
				// usually put your code
				run(ctx)
			},
			OnStoppedLeading: func() {
				// we can do cleanup here
				dlog.Infof(leaseCtx, "leader lost: %s", id)
				watchCancel()
			},
			OnNewLeader: func(identity string) {
				// we're notified when new leader elected
				if identity == id {
					// I just got the lock
					return
				}
				dlog.Infof(leaseCtx, "new leader elected: %s", identity)
			},
		},
	})

	err = grp.Wait()
	if err != nil {
		dlog.Error(ctx, err.Error())
	}
}

func getEnvWithDefault(envVarKey string, defaultValue string) string {
	value := os.Getenv(envVarKey)
	if value == "" {
		value = defaultValue
	}
	return value
}
