package cloudtoken_test

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	itest "github.com/datawire/ambassador-agent/integration_tests"
)

func (s *CloudTokenTestSuite) TestCloudToken() {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name() + "-cloud-token",
			Namespace: s.Namespace(),
		},
		Data: map[string][]byte{
			"CLOUD_CONNECT_TOKEN": []byte("TEST_TOKEN"),
		},
	}
	coreAPI := s.K8sIf().CoreV1()
	ctx := s.Context()
	_, err := coreAPI.Secrets(s.Namespace()).Create(ctx, &secret, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Cleanup(func(ctx context.Context) error {
		return coreAPI.Secrets(s.Namespace()).Delete(ctx, secret.ObjectMeta.Name, metav1.DeleteOptions{})
	})

	time.Sleep(15 * time.Second)

	pods, err := coreAPI.Pods(s.Namespace()).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=ambassador-agent",
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(pods.Items)
	agentPod := pods.Items[0]

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	logLines, err := itest.NewPodLogChan(ctx, s.K8sIf(), agentPod.Name, s.Namespace(), true)
	s.Require().NoError(err)

	var succ bool
	for line := range logLines {
		if succ = strings.Contains(line, "Setting cloud connect token from secret"); succ {
			break
		}
	}
	s.True(succ)
}
