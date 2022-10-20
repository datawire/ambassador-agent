package itest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/scheme"
	"github.com/datawire/ambassador-agent/pkg/api/agent"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const defaultKATServerImage = "docker.io/datawiredev/kat-server:3.0.1-0.20220817135951-2cb28ef4f415"

type CleanupFunc func(context.Context) error

type AgentCom struct {
	namespace      string
	name           string
	port           int
	clientset      *kubernetes.Clientset
	restConfig     *rest.Config
	katServerImage string
}

func NewAgentCom(name, ns string, restConfig *rest.Config) (*AgentCom, error) {
	if restConfig.APIPath == "" {
		restConfig.APIPath = "/api"
	}
	if restConfig.GroupVersion == nil {
		restConfig.GroupVersion = &schema.GroupVersion{
			Group:   "",
			Version: "v1",
		}
	}
	if restConfig.NegotiatedSerializer == nil {
		restConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{
			CodecFactory: scheme.Codecs,
		}
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &AgentCom{
		namespace:  ns,
		name:       name,
		restConfig: restConfig,
		clientset:  client,

		port:           8080,
		katServerImage: defaultKATServerImage,
	}, nil
}

func (ac *AgentCom) SetPort(p int) {
	ac.port = p
}

func (ac *AgentCom) SetKATServerImage(image string) {
	ac.katServerImage = image
}

func (ac *AgentCom) RPCAddress() string {
	return fmt.Sprintf("http://%s:%d", ac.name, ac.port)
}

func (ac *AgentCom) Install(ctx context.Context) (CleanupFunc, error) {
	_, err := ac.clientset.CoreV1().Namespaces().Get(ctx, ac.namespace, v1.GetOptions{})
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, err
		}

		_, err = ac.clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: ac.namespace,
			},
		}, v1.CreateOptions{})
		if err != nil {
			return nil, err
		}
	}

	svc := corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name: ac.name,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": ac.name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-server",
					Port:       int32(ac.port),
					TargetPort: intstr.FromString("grpc-server"),
				},
				{
					Name:       "snapshot-server",
					Port:       3001,
					TargetPort: intstr.FromString("snapshot-server"),
				},
			},
		},
	}

	pod := corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: ac.name,
			Labels: map[string]string{
				"app": ac.name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  ac.name,
					Image: ac.katServerImage,
					Env: []corev1.EnvVar{
						{
							Name:  "KAT_BACKEND_TYPE",
							Value: "grpc_agent",
						},
						{
							Name:  "KAT_GRPC_MAX_RECV_MSG_SIZE",
							Value: "65536", // 4 KiB
						},
					},
					Ports: []corev1.ContainerPort{
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
	}

	var client = ac.clientset.CoreV1()
	_, err = client.Services(ac.namespace).Create(ctx, &svc, v1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	_, err = client.Pods(ac.namespace).Create(ctx, &pod, v1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	cleanup := func(ctx context.Context) error {
		err := client.Services(ac.namespace).Delete(ctx, svc.Name, v1.DeleteOptions{})
		if err != nil {
			return nil
		}
		return client.Pods(ac.namespace).Delete(ctx, pod.Name, v1.DeleteOptions{})
	}

	return cleanup, err
}

func (ac *AgentCom) GetSnapshot(ctx context.Context) (*agent.Snapshot, error) {
	var snapshotBytes []byte

	for snapshotBytes == nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			outBytes, _, err := PodExec(ac.restConfig, ac.namespace, ac.name,
				"cat", "/tmp/snapshot.json",
			)
			if err != nil {
				if !strings.Contains(err.Error(), "exit code 1") {
					return nil, err
				}
			}

			if 0 < len(outBytes) {
				snapshotBytes = outBytes
			} else {
				time.Sleep(time.Second)
			}
		}
	}

	var ss agent.Snapshot
	if err := json.Unmarshal(snapshotBytes, &ss); err != nil {
		return nil, err
	}

	return &ss, nil
}
