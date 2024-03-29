package watchers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	v1networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type suiteNetworkWatcher struct {
	suite.Suite

	watcher *networkWatcher
}

func (s *suiteNetworkWatcher) BeforeTest() {
	s.watcher = &networkWatcher{
		watcher: nil,
		om:      nil,
	}
}

func (s *suiteNetworkWatcher) TestSpecConvertNoBackend() {
	// given
	ingress := &v1networking.Ingress{
		Spec: v1networking.IngressSpec{
			IngressClassName: nil,
			DefaultBackend:   nil,
			TLS:              nil,
			Rules:            nil,
		},
		Status: v1networking.IngressStatus{
			LoadBalancer: v1networking.IngressLoadBalancerStatus{
				Ingress: nil,
			},
		},
	}
	// when
	converted := s.watcher.convertSpec(ingress)

	// then
	assert.NotNil(s.T(), converted)
	assert.Equal(s.T(), extv1beta1.IngressSpec{
		IngressClassName: nil,
		Backend:          nil,
		TLS:              []extv1beta1.IngressTLS{},
		Rules:            []extv1beta1.IngressRule{},
	}, converted)
}

func (s *suiteNetworkWatcher) TestSpecConvertWithDefaultBackendAndRules() {
	// given
	ingress := &v1networking.Ingress{
		Spec: v1networking.IngressSpec{
			IngressClassName: nil,
			DefaultBackend: &v1networking.IngressBackend{
				Service: &v1networking.IngressServiceBackend{
					Name: "http",
					Port: v1networking.ServiceBackendPort{
						Name:   "http",
						Number: 80,
					},
				},
			},
			Rules: []v1networking.IngressRule{{
				Host: "www.example.com",
				IngressRuleValue: v1networking.IngressRuleValue{
					HTTP: &v1networking.HTTPIngressRuleValue{
						Paths: []v1networking.HTTPIngressPath{
							{
								Path:     "/apis",
								PathType: nil,
								Backend: v1networking.IngressBackend{
									Service: &v1networking.IngressServiceBackend{
										Name: "api",
										Port: v1networking.ServiceBackendPort{
											Name:   "http",
											Number: 80,
										},
									},
								},
							},
							{
								Path:     "/grpc",
								PathType: nil,
								Backend: v1networking.IngressBackend{
									Service: &v1networking.IngressServiceBackend{
										Name: "grpc",
										Port: v1networking.ServiceBackendPort{
											Number: 8080,
										},
									},
								},
							},
						},
					},
				},
			}},
			TLS: []v1networking.IngressTLS{
				{
					Hosts:      []string{"www.example.com"},
					SecretName: "my-certificate",
				},
			},
		},
		Status: v1networking.IngressStatus{
			LoadBalancer: v1networking.IngressLoadBalancerStatus{
				Ingress: nil,
			},
		},
	}
	// when
	converted := s.watcher.convertSpec(ingress)

	// then
	assert.NotNil(s.T(), converted)
	assert.Equal(s.T(), extv1beta1.IngressSpec{
		IngressClassName: nil,
		Backend: &extv1beta1.IngressBackend{
			ServiceName: "http",
			ServicePort: intstr.FromString("http"),
			Resource:    ingress.Spec.DefaultBackend.Resource,
		},
		TLS: []extv1beta1.IngressTLS{
			{
				Hosts:      []string{"www.example.com"},
				SecretName: "my-certificate",
			},
		},
		Rules: []extv1beta1.IngressRule{{
			Host: "www.example.com",
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: "/apis",
							Backend: extv1beta1.IngressBackend{
								ServiceName: "api",
								ServicePort: intstr.IntOrString{
									Type:   intstr.String,
									IntVal: 0,
									StrVal: "http",
								},
								Resource: nil,
							},
						},
						{
							Path: "/grpc",
							Backend: extv1beta1.IngressBackend{
								ServiceName: "grpc",
								ServicePort: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: 8080,
									StrVal: "",
								},
								Resource: nil,
							},
						},
					},
				},
			},
		}},
	}, converted)
}

func TestSuiteNetworkWatcher(t *testing.T) {
	suite.Run(t, new(suiteNetworkWatcher))
}
