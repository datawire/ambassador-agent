package aes_test

import (
	"context"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	itest "github.com/datawire/ambassador-agent/integration_tests"
)

func (s *AESTestSuite) TestAgentInstallation() {
	installationConfig := itest.InstallationConfig{
		ReleaseName: s.Name(),
		Namespace:   s.Namespace(),
		ChartDir:    "../../helm/ambassador-agent",
		Values: map[string]any{
			"rpcAddress": s.agentComServer.RPCAddress(),
		},

		RESTConfig: s.Config(),
		Log:        s.T().Logf,
	}
	ctx := s.Context()
	uninstallHelmChart, err := itest.InstallHelmChart(ctx, installationConfig)
	s.Require().NoError(err)
	s.Cleanup(uninstallHelmChart)

	time.Sleep(5 * time.Second)

	pods, err := s.K8sIf().CoreV1().Pods(s.Namespace()).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=ambassador-agent",
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(pods.Items)
	agentPod := pods.Items[0]

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	logLines, err := itest.NewPodLogChan(ctx, s.K8sIf(), agentPod.Name, s.Namespace(), true)
	s.Require().NoError(err)

	var succ bool
	for line := range logLines {
		if succ = strings.Contains(line, "Setting cloud connect token"); succ {
			break
		}
	}

	s.True(succ)
}
