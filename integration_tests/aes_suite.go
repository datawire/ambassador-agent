package itest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datawire/dlib/dexec"
)

func edgeStackHelmChartURL(version string) string {
	version = strings.TrimPrefix(version, "v")
	return fmt.Sprintf(
		"https://datawire-static-files.s3.amazonaws.com/charts/edge-stack-%s.tgz",
		version,
	)
}

type AESTestSuite struct {
	Suite
	agentComServer *AgentCom
}

func (s *AESTestSuite) SetupSuite() {
	s.Init()
	tempDir, err := os.MkdirTemp("", "")
	s.Require().NoError(err)

	agentImage := os.Getenv(agentImageEnvVar)
	s.Require().NotEmpty(agentImage,
		"%s needs to be set", agentImageEnvVar,
	)

	ctx := s.Context()
	s.Require().NoError(s.CreateNamespace(ctx, s.Namespace()))
	s.Cleanup(func(ctx context.Context) error {
		return s.DeleteNamespace(ctx, s.Namespace())
	})

	// install agentcom server
	s.agentComServer, err = NewAgentCom("agentcom-server", s.Namespace(), s.Config())
	s.Require().NoError(err)
	acCleanup, err := s.agentComServer.Install(ctx)
	s.Require().NoError(err)
	s.Cleanup(acCleanup)

	// install edge-stack
	crds := "https://app.getambassador.io/yaml/emissary/3.2.0/emissary-crds.yaml"
	cmd := dexec.CommandContext(ctx, "kubectl", "apply", "-f", crds)
	s.Require().NoError(cmd.Run())
	s.Cleanup(func(ctx context.Context) error {
		cmd := dexec.CommandContext(ctx, "kubectl", "delete", "-f", crds)
		return cmd.Run()
	})

	cmd = dexec.CommandContext(ctx, "kubectl", "-n", "emissary-system", "wait", "--timeout=90s", "--for=condition=available", "deployment", "emissary-apiext")
	s.Require().NoError(cmd.Run())

	aesChartPath := filepath.Join(tempDir, "edge-stack-8.1.0.tgz")
	file, err := os.Create(aesChartPath)
	s.Require().NoError(err)
	defer file.Close()

	resp, err := http.Get(edgeStackHelmChartURL("8.1.0"))
	s.Require().NoError(err)
	s.Require().Equal(resp.StatusCode, http.StatusOK)
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	s.Require().NoError(err)

	cmd = dexec.CommandContext(ctx, "tar", "xzf", "edge-stack-8.1.0.tgz")
	cmd.Dir = tempDir
	s.Require().NoError(cmd.Run())

	fmt.Printf("aesChartPath: %s\n\n", aesChartPath)

	installationConfig := InstallationConfig{
		ReleaseName: "aes",
		Namespace:   s.Namespace(),
		ChartDir:    filepath.Join(tempDir, "edge-stack"),
		Values: map[string]any{
			"agent": map[string]any{
				"cloudConnectToken": "TOKEN",
				"logLevel":          "debug",
				"rpcAddress":        s.agentComServer.RPCAddress(),
				"image": map[string]any{
					"fullImageOverride": agentImage,
				},
			},
		},
		RESTConfig: s.Config(),
		Log:        s.T().Logf,
	}
	uninstallHelmChart, err := InstallHelmChart(ctx, installationConfig)
	s.Require().NoError(err)
	s.Cleanup(uninstallHelmChart)

	time.Sleep(10 * time.Second)
}
