package agent_test

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/dtest"

	"github.com/emissary-ingress/emissary/v3/pkg/kates"
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

	// This env var is used by ./testdata/agent.yaml
	s.NotEmpty(os.Getenv("AMBASSADOR_AGENT_DOCKER_IMAGE"))

	s.resources = []any{
		apiv1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind: "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: s.namespace,
			},
		},
		apiv1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				Kind: "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.name,
				Namespace: s.namespace,
			},
		},
		rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: s.name,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{
						"namespaces",
						"services",
						"secrets",
						"configmaps",
					},
					Verbs: []string{
						"get",
						"list",
						"watch",
					},
				},
			},
		},
		rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: s.name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     s.name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      s.name,
					Namespace: s.namespace,
				},
			},
		},
		apiv1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.name + "-cloud-token",
				Namespace: s.namespace,
			},
			Data: map[string]string{
				"CLOUD_CONNECT_TOKEN": "sometoken",
			},
		},
		apiv1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind: "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.name,
				Namespace: s.namespace,
			},
			Spec: apiv1.PodSpec{
				ServiceAccountName: s.name,
				Containers: []apiv1.Container{
					{
						Image: os.Getenv("AMBASSADOR_AGENT_DOCKER_IMAGE"),
						Name:  s.name,
					},
				},
			},
		},
	}

	for _, resource := range s.resources {
		err := s.cli.Create(s.ctx, resource, nil)
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

func (s *BasicTestSuite) TestStandalone_StayAlive() {
	// lets make sure the agent came up and stays up
	time.Sleep(5 * time.Second)

	agentPod := apiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
		},
	}

	err := s.cli.Get(s.ctx, agentPod, &agentPod)
	s.Require().NoError(err)
	s.NotEmpty(agentPod.Status.ContainerStatuses)
	s.True(agentPod.Status.ContainerStatuses[0].Ready)

	logsReader, err := s.clientset.CoreV1().
		Pods(s.namespace).
		GetLogs(s.name, &apiv1.PodLogOptions{}).
		Stream(s.ctx)
	s.Require().NoError(err)

	logBytes, err := io.ReadAll(logsReader)
	s.Require().NoError(err)

	s.NotContains(string(logBytes), "rror")
}
