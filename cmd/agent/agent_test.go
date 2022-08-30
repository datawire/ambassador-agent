package agent_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/datawire/dlib/dlog"

	"github.com/emissary-ingress/emissary/v3/pkg/dtest"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
)

type BasicTestSuite struct {
	suite.Suite

	ctx context.Context

	cli       *kates.Client
	namespace string
	name      string

	resources []any
}

func TestBasicTestSuite(t *testing.T) {
	suite.Run(t, &BasicTestSuite{})
}

func (s *BasicTestSuite) SetupTest() {
	s.namespace = "ambassador-test"
	s.name = "ambassador-agent"
	s.ctx = dlog.NewTestContext(s.T(), false)

	kubeconfig := dtest.KubeVersionConfig(s.ctx, dtest.Kube22)

	var err error
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
						"pods",
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
		apiv1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				Kind: "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.name,
				Namespace: s.namespace,
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

	time.Sleep(3 * time.Second)
}

func (s *BasicTestSuite) TearDownSuite() {
	ctx := context.Background()
	for i := len(s.resources) - 1; 0 <= i; i-- {
		s.cli.Delete(ctx, s.resources[i], nil)
	}
}

func (s *BasicTestSuite) TestStandalone_StayAlive() {
	// lets make sure the agent came up and stays up
	time.Sleep(time.Second * 10)

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

	s.NoError(err)
}
