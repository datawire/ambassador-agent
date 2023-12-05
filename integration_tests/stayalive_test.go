package itest

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *BasicTestSuite) TestStayAlive() {
	pods, err := s.K8sIf().CoreV1().Pods(s.Namespace()).List(s.Context(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=ambassador-agent",
	})
	s.Require().NoError(err)
	s.Require().NotNil(pods)
	s.Require().NotEmpty(pods.Items)

	agentPod := pods.Items[0]
	s.NotEmpty(agentPod.Status.ContainerStatuses)
	s.True(agentPod.Status.ContainerStatuses[0].Ready)
}
