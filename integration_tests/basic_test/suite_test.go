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
	"k8s.io/apimachinery/pkg/util/intstr"
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

	var err error
	s.config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	s.Require().NoError(err)
	s.clientset, err = kubernetes.NewForConfig(s.config)
	s.Require().NoError(err)

	s.NotEmpty(os.Getenv(agentImageEnvVar),
		"%s needs to be set", agentImageEnvVar,
	)
	s.NotEmpty(os.Getenv(katImageEnvVar),
		"%s needs to be set", katImageEnvVar,
	)

	s.clientset.CoreV1().Namespaces().
		Create(s.ctx, &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: "ambassador-test",
			}},
			v1.CreateOptions{},
		)

	s.ensureAgentComServer()

	installationConfig := itest.InstallationConfig{
		ReleaseName: s.name,
		Namespace:   s.namespace,
		ChartDir:    "../../helm/ambassador-agent",
		Values: map[string]any{
			"cloudConnectToken": "TOKEN",
			"rpcAddress":        "http://agentcom-server:8080",
		},

		RESTConfig: s.config,
		Log:        s.T().Logf,
	}
	if 0 < len(s.namespaces) {
		installationConfig.Values["rbac"] = map[string]any{
			"namespaces": s.namespaces,
		}
	}
	s.uninstallHelmChart, err = itest.InstallHelmChart(s.ctx, installationConfig)
	s.Require().NoError(err)

	time.Sleep(10 * time.Second)
}

func (s *BasicTestSuite) TearDownSuite() {
	err := s.uninstallHelmChart()
	s.Require().NoError(err)

	s.clientset.CoordinationV1().Leases(s.namespace).
		Delete(s.ctx, "ambassador-agent-lease-lock", v1.DeleteOptions{})

	s.clientset.CoreV1().Services(s.namespace).
		Delete(s.ctx, "agentcom-server", v1.DeleteOptions{})

	s.clientset.CoreV1().Pods(s.namespace).
		Delete(s.ctx, "agentcom-server", v1.DeleteOptions{})

	time.Sleep(time.Second)
}

func (s *BasicTestSuite) ensureAgentComServer() {
	svc := corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name: "agentcom-server",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "agentcom-server",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-server",
					Port:       8080,
					TargetPort: intstr.FromString("grpc-server"),
				},
				{
					Name:       "snapshot-server",
					Port:       3001,
					TargetPort: intstr.FromString("snapshot-server"),
				},
			},
		},
	}

	pod := corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: "agentcom-server",
			Labels: map[string]string{
				"app": "agentcom-server",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "agentcom-server",
					Image: os.Getenv("KAT_SERVER_DOCKER_IMAGE"),
					Env: []corev1.EnvVar{
						{
							Name:  "KAT_BACKEND_TYPE",
							Value: "grpc_agent",
						},
						{
							Name:  "KAT_GRPC_MAX_RECV_MSG_SIZE",
							Value: "65536", // 4 KiB
						},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "grpc-server",
							ContainerPort: 8080,
						},
						{
							Name:          "snapshot-server",
							ContainerPort: 3001,
						},
					},
				},
			},
		},
	}

	var client = s.clientset.CoreV1()

	_, err := client.Services(s.namespace).Create(s.ctx, &svc, v1.CreateOptions{})
	s.Require().NoError(err)

	_, err = client.Pods(s.namespace).Create(s.ctx, &pod, v1.CreateOptions{})
	s.Require().NoError(err)
}
