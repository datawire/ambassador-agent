package basic_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/dtest"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	agentImageEnvVar = "AMBASSADOR_AGENT_DOCKER_IMAGE"
	katImageEnvVar   = "KAT_SERVER_DOCKER_IMAGE"
)

type BasicTestSuite struct {
	suite.Suite

	ctx context.Context

	cli       *kates.Client
	clientset *kubernetes.Clientset

	namespace string
	name      string

	resources []any
}

func TestBasicTestSuite(t *testing.T) {
	suite.Run(t, &BasicTestSuite{})
}

func (s *BasicTestSuite) SetupSuite() {
	s.namespace = "ambassador-test"
	s.name = "ambassador-agent"
	s.ctx = dlog.NewTestContext(s.T(), false)

	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	s.Require().NoError(err)
	s.clientset, err = kubernetes.NewForConfig(config)
	s.Require().NoError(err)

	kubeconfig := dtest.KubeVersionConfig(s.ctx, dtest.Kube22)
	s.cli, err = kates.NewClient(kates.ClientConfig{Kubeconfig: kubeconfig})
	s.NoError(err)

	s.NotEmpty(os.Getenv(agentImageEnvVar),
		"%s needs to be set", agentImageEnvVar,
	)
	s.NotEmpty(os.Getenv(katImageEnvVar),
		"%s needs to be set", katImageEnvVar,
	)

	s.resources = append(agentResources(s.namespace, s.name), fakeAgentcomResources(s.namespace)...)

	for _, resource := range s.resources {
		err := s.cli.Create(s.ctx, resource, nil)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				err = nil
			}
		}
		s.Require().NoError(err)
	}

	time.Sleep(10 * time.Second)
}

func (s *BasicTestSuite) TearDownSuite() {
	ctx := context.Background()
	for i := len(s.resources) - 1; 0 <= i; i-- {
		s.cli.Delete(ctx, s.resources[i], nil)
	}
}
