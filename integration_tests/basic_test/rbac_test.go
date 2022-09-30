package basic_test

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func (s *BasicTestSuite) TestRBAC() {
	roles, err := s.clientset.RbacV1().Roles("default").List(s.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + s.name,
	})
	s.Require().NoError(err)

	cr, err := s.clientset.RbacV1().ClusterRoles().Get(s.ctx, s.name, metav1.GetOptions{})
	if 0 < len(s.namespaces) {
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "not found")

		s.Require().Less(1, len(roles.Items))
	} else {
		s.Require().NoError(err)
		s.Require().NotNil(cr)

		s.Require().Equal(0, len(roles.Items))
	}
}
