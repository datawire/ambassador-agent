package aes_test

import (
	"context"
	"strings"
	"time"

	itest "github.com/datawire/ambassador-agent/integration_tests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *AESTestSuite) TestAgentInstallation() {
	installationConfig := itest.InstallationConfig{
		ReleaseName: s.name,
		Namespace:   s.namespace,
		ChartDir:    "../../helm/ambassador-agent",
		Values: map[string]any{
			"rpcAddress": s.agentComServer.RPCAddress(),
		},

		RESTConfig: s.config,
		Log:        s.T().Logf,
	}
	uninstallHelmChart, err := itest.InstallHelmChart(s.ctx, installationConfig)
	s.Require().NoError(err)
	s.cleanupFuncs["agent helm chart"] = uninstallHelmChart

	time.Sleep(5 * time.Second)

	pods, err := s.clientset.CoreV1().Pods(s.namespace).List(s.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=ambassador-agent",
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(pods.Items)
	agentPod := pods.Items[0]

	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	logLines, err := itest.NewPodLogChan(ctx, s.clientset, agentPod.Name, s.namespace, true)
	s.Require().NoError(err)

	var succ bool
	for line := range logLines {
		if succ = strings.Contains(line, "Setting cloud connect token"); succ {
			break
		}
	}

	s.True(succ)
}
