package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/cmd/exec"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	namespace     = "ambassador-dev"
	name          = "agentcom"
	rpcAddress    = "http://agentcom.ambassador-dev.svc.cluster.local:8080"
	annotationKey = "getambassador.io/dev-old-rpc-addresses"
)

type app struct {
	config *rest.Config
	cs     *kubernetes.Clientset

	agentImage string
}

func newApp(image string) (*app, error) {
	var (
		err error
		a   = app{agentImage: image}
	)

	a.cs, a.config, err = getClientSet()
	if err != nil {
		return nil, err
	}

	return &a, nil
}

func (a *app) isInstalled(ctx context.Context) (bool, error) {
	_, err := a.cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (a *app) ensureAgentCom(ctx context.Context) error {
	isInstalled, err := a.isInstalled(ctx)
	if err != nil {
		return fmt.Errorf("error detecting installation: %w", err)
	}
	if isInstalled {
		return nil
	}

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"getambassador.io/dev": name,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"getambassador.io/dev": name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-server",
					Port:       8080,
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
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"getambassador.io/dev": name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  name,
					Image: a.agentImage,
					Env: []corev1.EnvVar{
						{
							Name:  "KAT_BACKEND_TYPE",
							Value: "grpc_agent",
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

	var rollback = true
	if err := a.ensureNS(ctx); err != nil {
		return fmt.Errorf("unable to ensure 'ambassador-dev' namespace: %w", err)
	}

	_, err = a.cs.CoreV1().
		Pods(namespace).
		Create(ctx, &pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if rollback {
			a.cs.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		}
	}()

	_, err = a.cs.CoreV1().
		Services(namespace).
		Create(ctx, &svc, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	rollback = false

	return nil
}

func (a *app) uninstall(ctx context.Context) error {
	isInstalled, err := a.isInstalled(ctx)
	if err != nil {
		return fmt.Errorf("error detecting installation: %w", err)
	}
	if !isInstalled {
		return nil
	}

	var merr error
	err = a.cs.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("error deleting service: %w", err))
	}

	err = a.cs.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("error deleting pod: %w", err))
	}

	return merr
}

func (a *app) patchDeployment(ctx context.Context, ns, name string) error {
	dep, err := a.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting deployment: %w", err)
	}

	var oldAddresses = map[string]string{}
	for i, c := range dep.Spec.Template.Spec.Containers {
		var prev string
		dep.Spec.Template.Spec.Containers[i].Env, prev = patchedEnv(c.Env, rpcAddress)
		oldAddresses[c.Name] = prev
	}

	oldAddressesBytes, err := json.Marshal(oldAddresses)
	if err != nil {
		return fmt.Errorf("error marshalling old rpc connection addresses: %w", err)
	}

	dep.ObjectMeta.Annotations[annotationKey] = string(oldAddressesBytes)

	_, err = a.cs.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (a *app) unpatchDeployment(ctx context.Context, ns, name string) error {
	dep, err := a.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get deployment: %w", err)
	}

	oldAddresses := map[string]string{}
	oldAddressesJSON, ok := dep.ObjectMeta.Annotations[annotationKey]
	if !ok {
		return fmt.Errorf("deployment is missing annotation 'getambassador.io/dev-old-rpc-addresses'")
	}

	err = json.Unmarshal([]byte(oldAddressesJSON), &oldAddresses)
	if err != nil {
		return fmt.Errorf("unable to unmarshal old addresses")
	}

	for i, c := range dep.Spec.Template.Spec.Containers {
		oldAddress, ok := oldAddresses[c.Name]
		if !ok {
			continue
		}
		dep.Spec.Template.Spec.Containers[i].Env, _ = patchedEnv(c.Env, oldAddress)
	}

	delete(dep.ObjectMeta.Annotations, annotationKey)

	_, err = a.cs.AppsV1().Deployments(ns).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (a *app) snapshotBytes(ctx context.Context) ([]byte, error) {
	var snapshotBytes []byte

	for snapshotBytes == nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			outBytes, _, err := podExec(a.config,
				namespace, name,
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

	return snapshotBytes, nil
}

func (a *app) ensureNS(ctx context.Context) error {
	_, err := a.cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "not found") {
		return err
	}

	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err = a.cs.CoreV1().Namespaces().Create(ctx, &ns, metav1.CreateOptions{})
	return err
}

func getClientSet() (*kubernetes.Clientset, *rest.Config, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		return nil, nil, errors.New("KUBECONFIG not set")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return clientset, config, nil
}

func patchedEnv(env []corev1.EnvVar, newAddress string) ([]corev1.EnvVar, string) {
	var prev string
	nenv := make([]corev1.EnvVar, len(env))
	for i := range env {
		if env[i].Name == "RPC_CONNECTION_ADDRESS" {
			prev = env[i].Value
			nenv[i] = corev1.EnvVar{
				Name:  "RPC_CONNECTION_ADDRESS",
				Value: newAddress,
			}
		} else {
			nenv[i] = env[i]
		}
	}
	return nenv, prev
}

func podExec(config *rest.Config, ns, podName string, cmd ...string) (stdout, stderr []byte, err error) {
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
