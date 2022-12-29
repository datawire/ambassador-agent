package main

import (
	"context"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/datawire/ambassador-agent/pkg/agent"
	"github.com/datawire/dlib/dgroup"
	"github.com/datawire/dlib/dlog"
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

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		dlog.Error(ctx, err.Error())
		os.Exit(1)
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)
	ambAgent := agent.NewAgent(ctx, nil, agent.NewArgoRolloutsGetter, agent.NewSecretsGetter, clientset, env)

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

	grp.Go("watch", ambAgent.Watch)

	err = grp.Wait()
	if err != nil {
		dlog.Error(ctx, err.Error())
	}
}
