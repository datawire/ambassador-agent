package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/datawire/ambassador-agent/pkg/agent"
	"github.com/datawire/dlib/dgroup"
	"github.com/datawire/dlib/dlog"
)

// internal k8s service
const (
	AdminDiagnosticsPort     = 8877
	DefaultSnapshotURLFmt    = "http://ambassador-admin:%d/snapshot-external"
	DefaultDiagnosticsURLFmt = "http://ambassador-admin:%d/ambassador/v0/diag/?json=true"

	ExternalSnapshotPort = 8005
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	ctx := context.Background()
	ctx = dlog.WithLogger(ctx, dlog.WrapLogrus(logger))

	logLevel := getEnvWithDefault("LOG_LEVEL", "info")
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
		dlog.Errorf(ctx, "failed to parse log level %q : %v", logLevel, err)
	}
	logger.SetLevel(level)

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		dlog.Error(ctx, err.Error())
		os.Exit(1)
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)
	agentNamespace := getEnvWithDefault("AGENT_NAMESPACE", "ambassador")
	ambAgent := agent.NewAgent(ctx, nil, agent.NewArgoRolloutsGetter, agent.NewSecretsGetter, clientset, agentNamespace)

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

	grp.Go("watch", func(ctx context.Context) error {
		return ambAgent.Watch(ctx, snapshotURL, diagnosticsURL)
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
