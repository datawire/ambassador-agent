package itest

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/datawire/dlib/dlog"
)

func (s *AESTestSuite) TestAgentInstallation() {
	agentImage := os.Getenv(agentImageEnvVar)
	s.Require().NotEmpty(agentImage,
		"%s needs to be set", agentImageEnvVar,
	)
	installationConfig := InstallationConfig{
		ReleaseName: s.Name(),
		Namespace:   s.Namespace(),
		ChartDir:    "../helm/ambassador-agent",
		Values: map[string]any{
			"rpcAddress": s.agentComServer.RPCAddress(),
			"logLevel":   "debug",
			"image": map[string]any{
				"fullImageOverride": agentImage,
			},
		},

		RESTConfig: s.Config(),
		Log:        s.T().Logf,
	}
	ctx := s.Context()
	uninstallHelmChart, err := InstallHelmChart(ctx, installationConfig)
	s.Require().NoError(err)
	defer func() {
		s.NoError(uninstallHelmChart(ctx))
	}()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	logLines, err := NewPodLogChan(ctx, s.K8sIf(), AgentLabelSelector, s.Namespace(), true)
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
