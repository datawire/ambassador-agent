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
	ctx := s.Context()
	scAPI := s.K8sIf().CoreV1().Secrets(s.Namespace())
	_, err := scAPI.Create(ctx, &secret, metav1.CreateOptions{})
	s.Require().NoError(err)
	s.Cleanup(func(ctx context.Context) error {
		return scAPI.Delete(ctx, secret.ObjectMeta.Name, metav1.DeleteOptions{})
	})

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	logLines, err := itest.NewPodLogChan(ctx, s.K8sIf(), itest.AgentLabelSelector, s.Namespace(), true)
	s.Require().NoError(err)

	var succ bool
	for line := range logLines {
		if succ = strings.Contains(line, "Setting cloud connect token from secret"); succ {
			cancel()
			break
		}
	}
	s.True(succ)
}
