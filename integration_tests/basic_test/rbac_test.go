package basic_test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *BasicTestSuite) TestRBAC() {
	roles, err := s.clientset.RbacV1().Roles("default").List(s.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + s.name,
	})
	s.Require().NoError(err)

	cr, err := s.clientset.RbacV1().ClusterRoles().Get(s.ctx, s.name, metav1.GetOptions{})
	if 0 < len(s.namespaces) {
		s.Require().Error(err)
		s.Contains(err.Error(), "not found")

		s.Less(1, len(roles.Items))
	} else {
		s.Require().NoError(err)
		s.NotNil(cr)

		s.Equal(0, len(roles.Items))
	}
}
