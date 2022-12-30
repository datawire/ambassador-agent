package basic_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	itest "github.com/datawire/ambassador-agent/integration_tests"
)

const agentImageEnvVar = "AMBASSADOR_AGENT_DOCKER_IMAGE"

type BasicTestSuite struct {
	itest.Suite

	namespaces []string

	agentComServer *itest.AgentCom
}

func TestBasicTestSuite_Clusterwide(t *testing.T) {
	suite.Run(t, &BasicTestSuite{})
}

func TestBasicTestSuite_NamespaceScoped(t *testing.T) {
	suite.Run(t, &BasicTestSuite{
		namespaces: []string{"default"},
	})
}

func (s *BasicTestSuite) SetupSuite() {
	s.Init()
	s.NotEmpty(os.Getenv(agentImageEnvVar),
		"%s needs to be set", agentImageEnvVar,
	)

	ctx := s.Context()
	s.Require().NoError(s.CreateNamespace(ctx, s.Namespace()))
	s.Cleanup(func(ctx context.Context) error {
		return s.DeleteNamespace(ctx, s.Namespace())
	})

	var err error
	s.agentComServer, err = itest.NewAgentCom("agentcom-server", s.Namespace(), s.Config())
	s.Require().NoError(err)
	acCleanup, err := s.agentComServer.Install(ctx)
	s.Require().NoError(err)
	s.Cleanup(acCleanup)

	installationConfig := itest.InstallationConfig{
		ReleaseName: s.Name(),
		Namespace:   s.Namespace(),
		ChartDir:    "../../helm/ambassador-agent",
		Values: map[string]any{
			"cloudConnectToken": "TOKEN",
			"rpcAddress":        s.agentComServer.RPCAddress(),
		},

		RESTConfig: s.Config(),
		Log:        s.T().Logf,
	}
	if 0 < len(s.namespaces) {
		installationConfig.Values["rbac"] = map[string]any{
			"namespaces": s.namespaces,
		}
	}
	uninstallHelmChart, err := itest.InstallHelmChart(ctx, installationConfig)
	s.Require().NoError(err)
	s.Cleanup(uninstallHelmChart)
}
