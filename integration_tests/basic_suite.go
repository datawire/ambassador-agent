package itest

import (
	"context"
	"os"
	"time"
)

const agentImageEnvVar = "AMBASSADOR_AGENT_DOCKER_IMAGE"

type BasicTestSuite struct {
	Suite

	namespaces []string

	agentComServer *AgentCom
}

func (s *BasicTestSuite) SetupSuite() {
	s.Init()
	agentImage := os.Getenv(agentImageEnvVar)
	s.Require().NotEmpty(agentImage,
		"%s needs to be set", agentImageEnvVar,
	)

	ctx := s.Context()
	s.Require().NoError(s.CreateNamespace(ctx, s.Namespace()))
	s.Cleanup(func(ctx context.Context) error {
		return s.DeleteNamespace(ctx, s.Namespace())
	})

	var err error
	s.agentComServer, err = NewAgentCom("agentcom-server", s.Namespace(), s.Config())
	s.Require().NoError(err)
	acCleanup, err := s.agentComServer.Install(ctx)
	s.Require().NoError(err)
	s.Cleanup(acCleanup)
	time.Sleep(time.Second)

	installationConfig := InstallationConfig{
		ReleaseName: s.Name(),
		Namespace:   s.Namespace(),
		ChartDir:    "../helm/ambassador-agent",
		Values: map[string]any{
			"cloudConnectToken": "TOKEN",
			"logLevel":          "debug",
			"rpcAddress":        s.agentComServer.RPCAddress(),
			"image": map[string]any{
				"fullImageOverride": agentImage,
			},
		},

		RESTConfig: s.Config(),
		Log:        s.T().Logf,
	}
	if 0 < len(s.namespaces) {
		installationConfig.Values["rbac"] = map[string]any{
			"namespaces": s.namespaces,
		}
	}
	uninstallHelmChart, err := InstallHelmChart(ctx, installationConfig)
	s.Require().NoError(err)
	s.Cleanup(uninstallHelmChart)
}
