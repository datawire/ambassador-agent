package basic_test

import (
	"bufio"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *BasicTestSuite) TestCloudToken() {
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

	logLines := make(chan string, 1)
	go func() {
		logReader, err := s.clientset.CoreV1().Pods(s.namespace).
			GetLogs(agentPod.GetName(), &corev1.PodLogOptions{
				Follow: true,
			}).
			Stream(s.ctx)
		s.Require().NoError(err)
		scanner := bufio.NewScanner(logReader)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			logLines <- scanner.Text()
		}
	}()

	var (
		timeout = time.After(30 * time.Second)
		succ    bool
	)

OUTER:
	for {
		select {
		case <-timeout:
			break OUTER
		case line := <-logLines:
			if succ = strings.Contains(line, "Setting cloud connect token from secret"); succ {
				break OUTER
			}
		}
	}
	s.True(succ)
}
