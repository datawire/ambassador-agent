package itest

import (
	"context"
	"os"
)

const katImageEnvVar = "KAT_SERVER_DOCKER_IMAGE"

type CloudTokenTestSuite struct {
	Suite
}

func (s *CloudTokenTestSuite) SetupSuite() {
	s.Init()

	agentImage := os.Getenv(agentImageEnvVar)
	s.Require().NotEmpty(agentImage,
		"%s needs to be set", agentImageEnvVar,
	)
	s.NotEmpty(os.Getenv(katImageEnvVar),
		"%s needs to be set", katImageEnvVar,
	)

	installationConfig := InstallationConfig{
		ReleaseName: s.Name(),
		Namespace:   s.Namespace(),
		ChartDir:    "../helm/ambassador-agent",
		Values: map[string]any{
			"logLevel": "debug",
			"image": map[string]any{
				"fullImageOverride": agentImage,
			},
		},

		RESTConfig: s.Config(),
		Log:        s.T().Logf,
	}
	s.Require().NoError(s.CreateNamespace(s.Context(), s.Namespace()))
	s.Cleanup(func(ctx context.Context) error {
		return s.DeleteNamespace(ctx, s.Namespace())
	})

	uninstallHelmChart, err := InstallHelmChart(s.Context(), installationConfig)
	s.Require().NoError(err)
	s.Cleanup(uninstallHelmChart)
}
