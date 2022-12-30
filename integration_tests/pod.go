package itest

import (
	"bufio"
	"context"
	"io"
	"time"

	"github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/exec"

	"github.com/datawire/dlib/dlog"
	"github.com/datawire/dlib/dtime"
)

const AgentLabelSelector = "app.kubernetes.io/name=ambassador-agent"

// NewPodLogChan returns a chan with the specified pods logs entry-by-entry.
func NewPodLogChan(ctx context.Context, cs kubernetes.Interface, labelSelector, ns string, follow bool) (<-chan string, error) {
	client := cs.CoreV1().Pods(ns)
	opts := corev1.PodLogOptions{Follow: follow}
	var logReader io.ReadCloser
	for {
		names, err := RunningPods(ctx, cs, labelSelector, ns)
		if err != nil {
			return nil, err
		}
		if len(names) > 0 {
			if logReader, err = client.GetLogs(names[0], &opts).Stream(ctx); err != nil {
				return nil, err
			}
			break
		}
		dtime.SleepWithContext(ctx, 2*time.Second)
	}

	c := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(logReader)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() && ctx.Err() == nil {
			c <- scanner.Text()
		}
		close(c)
		logReader.Close()
	}()

	return c, nil
}

// RunningPods return the names of running pods with the given label selector in the form label=value. Running here means
// that at least one container is still running. I.e. the pod might well be terminating but still considered running.
func RunningPods(ctx context.Context, cs kubernetes.Interface, labelSelector, ns string) ([]string, error) {
	listOpts := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: "status.phase==Running",
	}
	pm, err := cs.CoreV1().Pods(ns).List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	pods := make([]string, 0, len(pm.Items))
nextPod:
	for _, pod := range pm.Items {
		for _, cn := range pod.Status.ContainerStatuses {
			if r := cn.State.Running; r != nil && !r.StartedAt.IsZero() {
				// At least one container is still running.
				pods = append(pods, pod.Name)
				continue nextPod
			}
		}
	}
	dlog.Infof(ctx, "Running pods %v", pods)
	return pods, nil
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
