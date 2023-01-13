package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/datawire/ambassador-agent/pkg/agent"
	"github.com/datawire/dlib/dgroup"
	"github.com/datawire/dlib/dlog"
	"github.com/datawire/k8sapi/pkg/k8sapi"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	ctx := context.Background()
	ctx = dlog.WithLogger(ctx, dlog.WrapLogrus(logger))
	env, err := agent.LoadEnv(os.LookupEnv)
	if err != nil {
		dlog.Error(ctx, err.Error())
		os.Exit(1)
	}

	logger.SetLevel(env.LogLevel)

	dlog.Infof(ctx, "ambassador-agent %s", agent.Version)

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		dlog.Error(ctx, err.Error())
		os.Exit(1)
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)
	ctx = k8sapi.WithK8sInterface(ctx, clientset)
	ambAgent := agent.NewAgent(ctx, nil, agent.NewArgoRolloutsGetter, agent.NewSecretsGetter, env)

	ambAgent.SetReportDiagnosticsAllowed(env.AESReportDiagnostics)

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

	// Begin lease-lock. watch when we are leader
	grp.Go("lease-lock-watch", func(ctx context.Context) error {
		// use a Go context so we can tell the leaderelection code when we
		// want to step down
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		// listen for interrupts or the Linux SIGTERM signal and cancel
		// our context, which the leader election code will observe and
		// step down
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-ch
			dlog.Info(ctx, "Received termination, signaling shutdown")
			cancel()
			os.Exit(0)
		}()

		// each call to the leaselock should have a unique id
		id := uuid.New().String()
		dlog.Infof(ctx, "Will lease with id %s", id)

		// we use the Lease lock type since edits to Leases are less common
		// and fewer objects in the cluster watch "all Leases".
		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      "ambassador-agent-lease-lock",
				Namespace: ambAgent.AgentNamespace,
			},
			Client: clientset.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: id,
			},
		}

		// check if we have permissions
		_, _, err = lock.Get(ctx)
		if err != nil {
			se := &apierrors.StatusError{}
			if errors.As(err, &se) && se.Status().Code == http.StatusForbidden {
				// if we do not have permissions, skip leader election
				dlog.Warnf(ctx, "Agent has no permissions to work with leases; will disable leader election. This may be inefficient. To fix, please install the agent from a new version of its helm chart")
				return ambAgent.Watch(ctx)
			} else {
				// This may be as simple as a not found
				dlog.Debugf(ctx, "Get lease failed: %v. Will try to start up regardless", err)
			}
		}

		// have a rotating context to stop watchers
		var (
			watchCancel context.CancelFunc
		)

		// start the leader election code loop
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
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
					dlog.Info(ctx, "Lease-lock aquired, watching cluster")
					ctx, watchCancel = context.WithCancel(ctx)
					ambAgent.Watch(ctx)
				},
				OnStoppedLeading: func() {
					// we can do cleanup here
					dlog.Info(ctx, "Lease-lock lost")
					if watchCancel != nil {
						watchCancel()
						watchCancel = nil
					}
				},
				OnNewLeader: func(identity string) {
					// we're notified when new leader elected
					if identity == id {
						// I just got the lock
						return
					}
					// uses lease-lock ctx, probably ok
					dlog.Infof(ctx, "a different agent aquired the lease-lock: %s", identity)
				},
			},
		})

		return nil
	})

	// grp.Go("agent-server", ambAgent.Service)

	err = grp.Wait()
	if err != nil {
		dlog.Error(ctx, err.Error())
	}
}
