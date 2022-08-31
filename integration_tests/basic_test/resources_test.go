package basic_test

import (
	"os"

	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func agentResources(namespace, name string) []any {
	return []any{
		apiv1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind: "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		},
		apiv1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				Kind: "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
		rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{
						"namespaces",
						"services",
						"secrets",
						"configmaps",
					},
					Verbs: []string{
						"get",
						"list",
						"watch",
					},
				},
			},
		},
		rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				Kind: "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      name,
					Namespace: namespace,
				},
			},
		},
		apiv1.Namespace{ // only needed for the configmap
			TypeMeta: metav1.TypeMeta{
				Kind: "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "ambassador",
			},
		},
		apiv1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind: "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name + "-cloud-token",
				Namespace: "ambassador",
			},
			Data: map[string]string{
				"CLOUD_CONNECT_TOKEN": "sometoken",
			},
		},
		apiv1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind: "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: apiv1.PodSpec{
				ServiceAccountName: name,
				Containers: []apiv1.Container{
					{
						Image: os.Getenv(agentImageEnvVar),
						Name:  name,
						Env: []apiv1.EnvVar{
							{
								Name:  "RPC_CONNECTION_ADDRESS",
								Value: "http://agentcom-server:8080/",
							},
						},
					},
				},
			},
		},
	}
}

func fakeAgentcomResources(namespace string) []any {
	return []any{
		apiv1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind: "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agentcom-server",
				Namespace: namespace,
				Labels: map[string]string{
					"app": "agentcom-server",
				},
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Image: os.Getenv(katImageEnvVar),
						Name:  "agentcom-server",
						Env: []apiv1.EnvVar{
							{
								Name:  "KAT_BACKED_TYPE",
								Value: "grpc_agent",
							},
							{
								Name:  "KAT_GRPC_MAX_RECV_MSG_SIZE",
								Value: "65536", //4KiB
							},
						},
						Ports: []apiv1.ContainerPort{
							{
								Name:          "grpc-server",
								ContainerPort: 8080,
							},
							{
								Name:          "snapshot-server",
								ContainerPort: 3001,
							},
						},
					},
				},
			},
		},
		apiv1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind: "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "agentcom-server",
				Namespace: namespace,
			},
			Spec: apiv1.ServiceSpec{
				Type: "ClusterIP",
				Selector: map[string]string{
					"app": "agentcom-server",
				},
				Ports: []apiv1.ServicePort{
					{
						Name:       "grpc-server",
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
					},
					{
						Name:       "snapshot-server",
						Port:       3001,
						TargetPort: intstr.FromInt(3001),
					},
				},
			},
		},
	}
}
