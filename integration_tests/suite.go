package itest

import (
	"context"
	"os"
	"time"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/dlib/dtime"
)

type Suite struct {
	suite.Suite
	ctx       context.Context
	config    *rest.Config
	k8sIf     kubernetes.Interface
	namespace string
	name      string
	cleanups  []func(context.Context) error
}

func (s *Suite) K8sIf() kubernetes.Interface {
	return s.k8sIf
}

func (s *Suite) Context() context.Context {
	return s.ctx
}

func (s *Suite) Config() *rest.Config {
	return s.config
}

func (s *Suite) Name() string {
	return s.name
}

func (s *Suite) Namespace() string {
	return s.namespace
}

func (s *Suite) Init() {
	s.name = "ambassador-agent"
	s.ctx = dlog.NewTestContext(s.T(), false)
	kubeconfigPath := os.Getenv("KUBECONFIG")
	s.Require().NotEmpty(kubeconfigPath, "%s needs to be set", "KUBECONFIG")

	var err error
	s.config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	s.Require().NoError(err)
	s.k8sIf, err = kubernetes.NewForConfig(s.config)
	s.Require().NoError(err)
}

func (s *Suite) Cleanup(f func(context.Context) error) {
	s.cleanups = append(s.cleanups, f)
}

func (s *Suite) TearDownSuite() {
	for {
		cs := s.cleanups
		s.cleanups = nil
		for i := len(cs) - 1; i >= 0; i-- {
			s.NoError(cs[i](s.ctx))
		}
		if len(s.cleanups) == 0 {
			break
		}
	}
}

func (s *Suite) CreateNamespace(ctx context.Context, ns string) error {
	_, err := s.k8sIf.CoreV1().Namespaces().Create(ctx,
		&corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: ns,
			},
		},
		v1.CreateOptions{},
	)
	return err
}

func (s *Suite) DeleteNamespace(ctx context.Context, ns string) error {
	nsAPI := s.k8sIf.CoreV1().Namespaces()
	if err := nsAPI.Delete(ctx, ns, v1.DeleteOptions{}); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	for {
		_, err := nsAPI.Get(ctx, ns, v1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				err = nil
			}
			return err
		}
		dtime.SleepWithContext(ctx, 5*time.Second)
	}
}
