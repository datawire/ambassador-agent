package cloudtoken_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	itest "github.com/datawire/ambassador-agent/integration_tests"
	"github.com/datawire/dlib/dlog"
)

const (
	agentImageEnvVar = "AMBASSADOR_AGENT_DOCKER_IMAGE"
	katImageEnvVar   = "KAT_SERVER_DOCKER_IMAGE"
)

type CloudTokenTestSuite struct {
	suite.Suite

	ctx context.Context

	clientset *kubernetes.Clientset

	namespace string
	name      string

	uninstallHelmChart itest.CleanupFunc
}

func Test_Run(t *testing.T) {
	suite.Run(t, &CloudTokenTestSuite{})
}

func (s *CloudTokenTestSuite) SetupSuite() {
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

	installationConfig := itest.InstallationConfig{
		ReleaseName: s.name,
		Namespace:   s.namespace,
		ChartDir:    "../../helm/ambassador-agent",

		RESTConfig: config,
		Log:        s.T().Logf,
	}
	s.uninstallHelmChart, err = itest.InstallHelmChart(s.ctx, installationConfig)
	s.Require().NoError(err)

	time.Sleep(10 * time.Second)
}

func (s *CloudTokenTestSuite) TearDownSuite() {
	err := s.uninstallHelmChart(s.ctx)
	s.Require().NoError(err)

	_ = s.clientset.CoordinationV1().Leases(s.namespace).
		Delete(s.ctx, "ambassador-agent-lease-lock", v1.DeleteOptions{})

	time.Sleep(time.Second)
}
