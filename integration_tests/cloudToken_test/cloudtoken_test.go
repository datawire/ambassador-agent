package cloudtoken_test

import (
	"context"
	"strings"
	"time"

	itest "github.com/datawire/ambassador-agent/integration_tests"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *CloudTokenTestSuite) TestCloudToken() {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name + "-cloud-token",
			Namespace: s.namespace,
		},
		Data: map[string][]byte{
			"CLOUD_CONNECT_TOKEN": []byte("TEST_TOKEN"),
		},
	}
	_, err := s.clientset.CoreV1().Secrets(s.namespace).
		Create(s.ctx, &secret, metav1.CreateOptions{})
	s.Require().NoError(err)

	defer func() {
		s.clientset.CoreV1().Secrets(s.namespace).Delete(s.ctx, secret.ObjectMeta.Name, metav1.DeleteOptions{})
	}()

	time.Sleep(15 * time.Second)

	pods, err := s.clientset.CoreV1().Pods(s.namespace).List(s.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=ambassador-agent",
	})
	s.Require().NoError(err)
	s.Require().NotEmpty(pods.Items)
	agentPod := pods.Items[0]

	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()

	logLines, err := itest.NewPodLogChan(ctx, s.clientset, agentPod.Name, s.namespace, true)
	s.Require().NoError(err)

	var succ bool
	for line := range logLines {
		if succ = strings.Contains(line, "Setting cloud connect token from secret"); succ {
			break
		}
	}
	s.True(succ)
}
