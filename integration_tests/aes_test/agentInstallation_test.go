package aes_test

import (
	"context"
	"strings"
	"time"

	itest "github.com/datawire/ambassador-agent/integration_tests"
	"github.com/datawire/dlib/dlog"
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

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	logLines, err := itest.NewPodLogChan(ctx, s.K8sIf(), itest.AgentLabelSelector, s.Namespace(), true)
	s.Require().NoError(err)

	var succ bool
	for line := range logLines {
		dlog.Info(ctx, line)
		if succ = strings.Contains(line, "Setting cloud connect token from environment"); succ {
			cancel()
			break
		}
	}
	s.True(succ)
}
