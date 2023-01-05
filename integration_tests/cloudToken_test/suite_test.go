package cloudtoken_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	itest "github.com/datawire/ambassador-agent/integration_tests"
)

const (
	agentImageEnvVar = "AMBASSADOR_AGENT_DOCKER_IMAGE"
	katImageEnvVar   = "KAT_SERVER_DOCKER_IMAGE"
)

type CloudTokenTestSuite struct {
	itest.Suite
}

func Test_Run(t *testing.T) {
	suite.Run(t, &CloudTokenTestSuite{})
}

func (s *CloudTokenTestSuite) SetupSuite() {
	s.Init()

	s.NotEmpty(os.Getenv(agentImageEnvVar),
		"%s needs to be set", agentImageEnvVar,
	)
	s.NotEmpty(os.Getenv(katImageEnvVar),
		"%s needs to be set", katImageEnvVar,
	)

	installationConfig := itest.InstallationConfig{
		ReleaseName: s.Name(),
		Namespace:   s.Namespace(),
		ChartDir:    "../../helm/ambassador-agent",

		RESTConfig: s.Config(),
		Log:        s.T().Logf,
	}
	s.Require().NoError(s.CreateNamespace(s.Context(), s.Namespace()))
	s.Cleanup(func(ctx context.Context) error {
		return s.DeleteNamespace(ctx, s.Namespace())
	})

	uninstallHelmChart, err := itest.InstallHelmChart(s.Context(), installationConfig)
	s.Require().NoError(err)
	s.Cleanup(uninstallHelmChart)
}
