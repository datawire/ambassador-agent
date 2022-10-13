package basic_test

import (
	"context"
	"os"
	"testing"
	"time"

	itest "github.com/datawire/ambassador-agent/integration_tests"
	"github.com/datawire/dlib/dlog"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	config    *rest.Config
	clientset *kubernetes.Clientset

	namespace string
	name      string

	namespaces []string

	cleanupFuncs   map[string]itest.CleanupFunc
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
	s.cleanupFuncs = make(map[string]itest.CleanupFunc)
	s.namespace = "ambassador-test"
	s.name = "ambassador-agent"
	s.ctx = dlog.NewTestContext(s.T(), false)

	kubeconfigPath := os.Getenv("KUBECONFIG")
	s.Require().NotEmpty(kubeconfigPath)

	var err error
	s.config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	s.Require().NoError(err)
	s.clientset, err = kubernetes.NewForConfig(s.config)
	s.Require().NoError(err)

	s.NotEmpty(os.Getenv(agentImageEnvVar),
		"%s needs to be set", agentImageEnvVar,
	)

	s.clientset.CoreV1().Namespaces().
		Create(s.ctx, &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: "ambassador-test",
			}},
			v1.CreateOptions{},
		)

	s.agentComServer, err = itest.NewAgentCom("agentcom-server", s.namespace, s.config)
	s.Require().NoError(err)
	acCleanup, err := s.agentComServer.Install(s.ctx)
	s.Require().NoError(err)
	s.cleanupFuncs["agentcom server"] = acCleanup

	installationConfig := itest.InstallationConfig{
		ReleaseName: s.name,
		Namespace:   s.namespace,
		ChartDir:    "../../helm/ambassador-agent",
		Values: map[string]any{
			"cloudConnectToken": "TOKEN",
			"rpcAddress":        s.agentComServer.RPCAddress(),
		},

		RESTConfig: s.config,
		Log:        s.T().Logf,
	}
	if 0 < len(s.namespaces) {
		installationConfig.Values["rbac"] = map[string]any{
			"namespaces": s.namespaces,
		}
	}
	uninstallHelmChart, err := itest.InstallHelmChart(s.ctx, installationConfig)
	s.Require().NoError(err)
	s.cleanupFuncs["agent helm chart"] = uninstallHelmChart

	time.Sleep(10 * time.Second)
}

func (s *BasicTestSuite) TearDownSuite() {
	for name, f := range s.cleanupFuncs {
		if err := f(s.ctx); err != nil {
			s.T().Logf("error cleaning up %s: %s", name, err.Error())
		}
	}

	// left over from the helm chart installation
	s.clientset.CoordinationV1().Leases(s.namespace).
		Delete(s.ctx, "ambassador-agent-lease-lock", v1.DeleteOptions{})

	time.Sleep(time.Second)
}
