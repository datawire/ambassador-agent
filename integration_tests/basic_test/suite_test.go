package basic_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/datawire/dlib/dlog"
	"github.com/stretchr/testify/suite"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	agentImageEnvVar = "AMBASSADOR_AGENT_DOCKER_IMAGE"
	katImageEnvVar   = "KAT_SERVER_DOCKER_IMAGE"
)

type BasicTestSuite struct {
	suite.Suite

	ctx context.Context

	clientset *kubernetes.Clientset

	namespace string
	name      string

	namespaces []string

	uninstallHelmChart func() error
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
	s.namespace = "ambassador-test"
	s.name = "ambassador-agent"
	s.ctx = dlog.NewTestContext(s.T(), false)

	kubeconfigPath := os.Getenv("KUBECONFIG")
	s.Require().NotEmpty(kubeconfigPath)

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	s.Require().NoError(err)
	s.clientset, err = kubernetes.NewForConfig(config)
	s.Require().NoError(err)

	s.NotEmpty(os.Getenv(agentImageEnvVar),
		"%s needs to be set", agentImageEnvVar,
	)
	s.NotEmpty(os.Getenv(katImageEnvVar),
		"%s needs to be set", katImageEnvVar,
	)

	s.uninstallHelmChart, err = s.installHelmChart(config)
	s.Require().NoError(err)

	time.Sleep(10 * time.Second)
}

func (s *BasicTestSuite) TearDownSuite() {
	err := s.uninstallHelmChart()
	s.Require().NoError(err)

	s.clientset.CoordinationV1().Leases(s.namespace).
		Delete(s.ctx, "ambassador-agent-lease-lock", v1.DeleteOptions{})

	time.Sleep(time.Second)
}

func (s *BasicTestSuite) installHelmChart(config *rest.Config) (uninstall func() error, err error) {
	var (
		helmCliConfig = genericclioptions.NewConfigFlags(false)
		actionConfig  action.Configuration
	)

	helmCliConfig.APIServer = &config.Host
	helmCliConfig.BearerToken = &config.BearerToken
	helmCliConfig.CAFile = &config.CAFile
	helmCliConfig.Namespace = &s.namespace

	err = actionConfig.Init(helmCliConfig, s.namespace, "", s.T().Logf)
	if err != nil {
		return nil, err
	}

	chart, err := loader.LoadDir("../../helm/ambassador-agent")
	if err != nil {
		return nil, err
	}

	install := action.NewInstall(&actionConfig)
	install.ReleaseName = s.name
	install.Namespace = s.namespace
	install.CreateNamespace = true

	vals := map[string]any{}
	if 0 < len(s.namespaces) {
		vals["rbac"] = map[string]any{
			"namespaces": s.namespaces,
		}
	}
	release, err := install.RunWithContext(s.ctx, chart, vals)
	if err != nil {
		return nil, err
	}

	return func() error {
		_, err := action.NewUninstall(&actionConfig).Run(release.Name)
		return err
	}, nil

}
