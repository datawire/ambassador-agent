package itest

import (
	"github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/scheme"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/exec"
)

func Exec(config *rest.Config, ns, podName string, cmd ...string) (stdout, stderr []byte, err error) {
	if config.APIPath == "" {
		config.APIPath = "/api"
	}
	if config.GroupVersion == nil {
		config.GroupVersion = &schema.GroupVersion{
			Group:   "",
			Version: "v1",
		}
	}
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	streams, _, outBuf, errBuf := genericclioptions.NewTestIOStreams()
	execOpts := exec.ExecOptions{
		StreamOptions: exec.StreamOptions{
			Namespace: ns,
			PodName:   podName,
			IOStreams: streams,
		},
		Config:    config,
		Command:   cmd,
		PodClient: client.CoreV1(),
		Executor:  &exec.DefaultRemoteExecutor{},
	}

	if err := execOpts.Run(); err != nil {
		return nil, nil, err
	}

	return outBuf.Bytes(), errBuf.Bytes(), nil
}
