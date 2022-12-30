package itest

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/scheme"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/exec"

	"github.com/datawire/dlib/dtime"
)

// NewPodLogChan returns a chan with the specified pods logs entry-by-entry.
func NewPodLogChan(ctx context.Context, cs kubernetes.Interface, name, ns string, follow bool) (<-chan string, error) {
	var (
		client = cs.CoreV1().Pods(ns)
		opts   = corev1.PodLogOptions{Follow: follow}
	)

	var logReader io.ReadCloser
	for {
		var err error
		if logReader, err = client.GetLogs(name, &opts).Stream(ctx); err != nil {
			if strings.Contains(err.Error(), "waiting to start") {
				dtime.SleepWithContext(ctx, 5*time.Second)
				continue
			}
			return nil, err
		}
		break
	}

	var (
		c       = make(chan string, 1)
		scanner = bufio.NewScanner(logReader)
	)
	scanner.Split(bufio.ScanLines)

	go func() {
		for scanner.Scan() && ctx.Err() == nil {
			c <- scanner.Text()
		}
		close(c)
		logReader.Close()
	}()

	return c, nil
}

// PodExec provides the functionality of `kubectl exec -n ns podName -- cmd...`.
func PodExec(config *rest.Config, ns, podName string, cmd ...string) (stdout, stderr []byte, err error) {
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
		return nil, errBuf.Bytes(), err
	}

	return outBuf.Bytes(), errBuf.Bytes(), nil
}
