package basic_test

import (
	"io"
	"time"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *BasicTestSuite) TestStayAlive() {
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
