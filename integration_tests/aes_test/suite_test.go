package aes_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	itest "github.com/datawire/ambassador-agent/integration_tests"
	"github.com/datawire/dlib/dlog"
)

const (
	agentImageEnvVar = "AMBASSADOR_AGENT_DOCKER_IMAGE"
	katImageEnvVar   = "KAT_SERVER_DOCKER_IMAGE"
)

func edgeStackHelmChartURL(version string) string {
	version = strings.TrimPrefix(version, "v")
	return fmt.Sprintf(
		"https://datawire-static-files.s3.amazonaws.com/charts/edge-stack-%s.tgz",
		version,
	)
}

type AESTestSuite struct {
	suite.Suite

	ctx context.Context

	config    *rest.Config
	clientset *kubernetes.Clientset

	tempDir string

	namespace string
	name      string

	cleanupFuncs   map[string]itest.CleanupFunc
	agentComServer *itest.AgentCom
}

func TestBasicTestSuite_Clusterwide(t *testing.T) {
	suite.Run(t, &AESTestSuite{})
}

func (s *AESTestSuite) SetupSuite() {
	s.cleanupFuncs = make(map[string]itest.CleanupFunc)
	s.namespace = "ambassador-test"
	s.name = "ambassador-agent"
	s.ctx = dlog.NewTestContext(s.T(), false)
	s.tempDir, _ = os.MkdirTemp("", "")

	kubeconfigPath := os.Getenv("KUBECONFIG")
	s.Require().NotEmpty(kubeconfigPath)

	var err error
	s.config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	s.Require().NoError(err)
	s.clientset, err = kubernetes.NewForConfig(s.config)
	s.Require().NoError(err)

	s.Require().NotEmpty(os.Getenv(agentImageEnvVar),
		"%s needs to be set", agentImageEnvVar,
	)

	// install agentcom server
	s.agentComServer, err = itest.NewAgentCom("agentcom-server", s.namespace, s.config)
	s.Require().NoError(err)
	acCleanup, err := s.agentComServer.Install(s.ctx)
	s.Require().NoError(err)
	s.cleanupFuncs["agentcom server"] = acCleanup

	// install edge-stack
	cmd := exec.Command("kubectl", "apply", "-f", "https://app.getambassador.io/yaml/emissary/3.2.0/emissary-crds.yaml")
	_, err = cmd.CombinedOutput()
	s.Require().NoError(err)

	cmd = exec.Command("kubectl", "-n", "emissary-system", "wait", "--timeout=90s", "--for=condition=available", "deployment", "emissary-apiext")
	_, err = cmd.CombinedOutput()
	s.Require().NoError(err)

	aesChartPath := filepath.Join(s.tempDir, "edge-stack-8.1.0.tgz")
	file, err := os.Create(aesChartPath)
	s.Require().NoError(err)
	defer file.Close()

	resp, err := http.Get(edgeStackHelmChartURL("8.1.0"))
	s.Require().NoError(err)
	s.Require().Equal(resp.StatusCode, http.StatusOK)
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	s.Require().NoError(err)

	cmd = exec.Command("tar", "xzf", "edge-stack-8.1.0.tgz")
	cmd.Dir = s.tempDir
	_, err = cmd.CombinedOutput()
	s.Require().NoError(err)

	fmt.Printf("aesChartPath: %s\n\n", aesChartPath)

	installationConfig := itest.InstallationConfig{
		ReleaseName: "aes",
		Namespace:   s.namespace,
		ChartDir:    filepath.Join(s.tempDir, "edge-stack"),
		Values: map[string]any{
			"agent": map[string]any{
				"cloudConnectToken": "TOKEN",
				"rpcAddress":        s.agentComServer.RPCAddress(),
			},
		},
		RESTConfig: s.config,
		Log:        s.T().Logf,
	}
	uninstallHelmChart, err := itest.InstallHelmChart(s.ctx, installationConfig)
	s.Require().NoError(err)
	s.cleanupFuncs["aes helm chart"] = uninstallHelmChart

	time.Sleep(10 * time.Second)
}

func (s *AESTestSuite) TearDownSuite() {
	for name, f := range s.cleanupFuncs {
		if err := f(s.ctx); err != nil {
			s.T().Logf("error cleaning up %s: %s", name, err.Error())
		}
	}

	cmd := exec.Command("kubectl", "delete", "-f", "https://app.getambassador.io/yaml/emissary/3.2.0/emissary-crds.yaml")
	_, err := cmd.CombinedOutput()
	s.Require().NoError(err)

	time.Sleep(time.Second)
}
