package itest

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *BasicTestSuite) TestRBAC() {
	rbacIF := s.K8sIf().RbacV1()
	ctx := s.Context()
	roles, err := rbacIF.Roles("default").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + s.Name(),
	})
	s.Require().NoError(err)

	cr, err := rbacIF.ClusterRoles().Get(ctx, s.Name(), metav1.GetOptions{})
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
