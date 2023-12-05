package itest

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *CloudTokenTestSuite) TestCloudTokenWithSecret() {
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
	defer func() {
		s.NoError(scAPI.Delete(s.Context(), secret.ObjectMeta.Name, metav1.DeleteOptions{}))
	}()

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	logLines, err := NewPodLogChan(ctx, s.K8sIf(), AgentLabelSelector, s.Namespace(), true)
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

func (s *CloudTokenTestSuite) TestCloudTokenWithConfigMap() {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name() + "-cloud-token",
			Namespace: s.Namespace(),
		},
		Data: map[string]string{
			"CLOUD_CONNECT_TOKEN": "TEST_TOKEN",
		},
	}

	ctx := s.Context()
	scAPI := s.K8sIf().CoreV1().ConfigMaps(s.Namespace())
	_, err := scAPI.Create(ctx, &cm, metav1.CreateOptions{})
	s.Require().NoError(err)
	defer func() {
		s.NoError(scAPI.Delete(s.Context(), cm.ObjectMeta.Name, metav1.DeleteOptions{}))
	}()

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	logLines, err := NewPodLogChan(ctx, s.K8sIf(), AgentLabelSelector, s.Namespace(), true)
	s.Require().NoError(err)

	var succ bool
	for line := range logLines {
		if succ = strings.Contains(line, "Setting cloud connect token from configmap"); succ {
			cancel()
			break
		}
	}
	s.True(succ)
}

func (s *CloudTokenTestSuite) TestNoCloudToken() {
	ctx := s.Context()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	logLines, err := NewPodLogChan(ctx, s.K8sIf(), AgentLabelSelector, s.Namespace(), true)
	s.Require().NoError(err)

	var succ bool
	for line := range logLines {
		if succ = strings.Contains(line, "Unable to get cloud connect token. This agent will do nothing."); succ {
			cancel()
			break
		}
	}
	s.True(succ)
}
