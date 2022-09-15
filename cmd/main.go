package main

import (
	"context"
	"flag"
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
	"k8s.io/klog/v2"

	"github.com/datawire/dlib/dgroup"
	"github.com/datawire/dlib/dlog"
	"github.com/emissary-ingress/emissary/v3/pkg/busy"
	"github.com/emissary-ingress/emissary/v3/pkg/logutil"
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
		klog.Fatal(err.Error())
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)

	agentNamespace := getEnvWithDefault("AGENT_NAMESPACE", "ambassador")

	ambAgent := agent.NewAgent(nil, agent.NewArgoRolloutsGetter, agent.NewSecretsGetter, clientset, agentNamespace)

	// all log things need to happen here because we still allow the agent to run in amb-sidecar
	// and amb-sidecar should control all the logging if it's kicking off the agent.
	// this codepath is only hit when the agent is running on its own
	logLevel := os.Getenv("AES_LOG_LEVEL")
	// by default, suppress everything except fatal things
	// the watcher in the agent will spit out a lot of errors because we don't give it rbac to
	// list secrets initially.
	klogLevel := 3
	if logLevel != "" {
		logrusLevel, err := logutil.ParseLogLevel(logLevel)
		if err != nil {
			dlog.Errorf(ctx, "error parsing log level, running with default level: %+v", err)
		} else {
			busy.SetLogLevel(logrusLevel)
		}
		klogLevel = logutil.LogrusToKLogLevel(logrusLevel)
	}
	klogFlags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	klog.InitFlags(klogFlags)
	if err := klogFlags.Parse([]string{fmt.Sprintf("-stderrthreshold=%d", klogLevel), "-v=2", "-logtostderr=false"}); err != nil {
		klog.Fatal(err.Error())
	}
	snapshotURL := getEnvWithDefault("AES_SNAPSHOT_URL", fmt.Sprintf(DefaultSnapshotURLFmt, ExternalSnapshotPort))
	diagnosticsURL := getEnvWithDefault("AES_DIAGNOSTICS_URL", fmt.Sprintf(DefaultDiagnosticsURLFmt, AdminDiagnosticsPort))

	reportDiagnostics := os.Getenv("AES_REPORT_DIAGNOSTICS_TO_CLOUD")
	if reportDiagnostics == "true" {
		ambAgent.SetReportDiagnosticsAllowed(true)
	}

	metricsListener, err := net.Listen("tcp", ":8080")
	if err != nil {
		klog.Fatal(err.Error())
	}
	dlog.Info(ctx, "metrics service listening on :8080")

	grp := dgroup.NewGroup(ctx, dgroup.GroupConfig{})
	grp.Go("metrics-server", func(ctx context.Context) error {
		metricsServer := agent.NewMetricsServer(ambAgent.MetricsRelayHandler)
		return metricsServer.Serve(ctx, metricsListener)
	})

	// use a Go context so we can tell the leaderelection code when we
	// want to step down
	leaseCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// listen for interrupts or the Linux SIGTERM signal and cancel
	// our context, which the leader election code will observe and
	// step down
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()

	// each call to the leaselock should have a unique id
	id := uuid.New().String()

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
	var watchCtx context.Context
	var watchCancel context.CancelFunc
	run := func() {
		grp.Go("watch", func(ctx context.Context) error {
			watchCtx, watchCancel = context.WithCancel(ctx)
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
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// we're notified when we start - this is where you would
				// usually put your code
				run()
			},
			OnStoppedLeading: func() {
				// we can do cleanup here
				klog.Infof("leader lost: %s", id)
				watchCancel()
			},
			OnNewLeader: func(identity string) {
				// we're notified when new leader elected
				if identity == id {
					// I just got the lock
					return
				}
				klog.Infof("new leader elected: %s", identity)
			},
		},
	})

	err = grp.Wait()
	if err != nil {
		klog.Fatal(err.Error())
	}
}

func getEnvWithDefault(envVarKey string, defaultValue string) string {
	value := os.Getenv(envVarKey)
	if value == "" {
		value = defaultValue
	}
	return value
}
