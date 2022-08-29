package agent_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/datawire/dlib/dexec"
	"github.com/datawire/dlib/dlog"

	"github.com/emissary-ingress/emissary/v3/pkg/dtest"
	"github.com/emissary-ingress/emissary/v3/pkg/k8s"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"github.com/emissary-ingress/emissary/v3/pkg/kubeapply"
)

func TestStandalone_StayAlive(t *testing.T) {
	ctx := dlog.NewTestContext(t, false)
	kubeconfig := dtest.KubeVersionConfig(ctx, dtest.Kube22)
	cli, err := kates.NewClient(kates.ClientConfig{Kubeconfig: kubeconfig})
	require.NoError(t, err)

	setup(t, ctx, kubeconfig, cli)

	// lets make sure the agent came up and stays up
	time.Sleep(time.Second * 10)

	agentPod := apiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ambassador-agent",
			Namespace: "ambassador",
		},
	}
	err = cli.Get(ctx, agentPod, &agentPod)
	require.NoError(t, err)
	require.NotEmpty(t, agentPod.Labels)
}

func needsDockerBuilds(ctx context.Context, var2file map[string]string) error {
	// TODO(lukeshu): Consider unifying envoytest.GetLocalEnvoyImage() with
	// agent_test.go:needsDockerBuilds().
	var targets []string
	for varname, filename := range var2file {
		if os.Getenv(varname) == "" {
			targets = append(targets, filename)
		}
	}
	if len(targets) == 0 {
		return nil
	}
	if os.Getenv("DEV_REGISTRY") == "" {
		registry := dtest.DockerRegistry(ctx)
		os.Setenv("DEV_REGISTRY", registry)
		os.Setenv("DTEST_REGISTRY", registry)
	}
	cmdline := append([]string{"make", "-C", "../.."}, targets...)
	if err := dexec.CommandContext(ctx, cmdline[0], cmdline[1:]...).Run(); err != nil {
		return err
	}
	for varname, filename := range var2file {
		if os.Getenv(varname) == "" {
			dat, err := ioutil.ReadFile(filepath.Join("../..", filename))
			if err != nil {
				return err
			}
			lines := strings.Split(strings.TrimSpace(string(dat)), "\n")
			if len(lines) < 2 {
				return fmt.Errorf("malformed docker.mk tagfile %q", filename)
			}
			if err := os.Setenv(varname, lines[1]); err != nil {
				return err
			}
		}
	}
	return nil
}

func setup(t *testing.T, ctx context.Context, kubeconfig string, cli *kates.Client) {
	require.NoError(t, needsDockerBuilds(ctx, map[string]string{
		"AMBASSADOR_AGENT_DOCKER_IMAGE": "docker/emissary.docker.push.remote",
		"KAT_SERVER_DOCKER_IMAGE":       "docker/kat-server.docker.push.remote",
	}))

	image := os.Getenv("AMBASSADOR_AGENT_DOCKER_IMAGE")
	require.NotEmpty(t, image)

	kubeinfo := k8s.NewKubeInfo(kubeconfig, "", "")

	require.NoError(t, kubeapply.Kubeapply(ctx, kubeinfo, time.Minute, true, false, "./testdata/namespace.yaml"))
	require.NoError(t, kubeapply.Kubeapply(ctx, kubeinfo, 2*time.Minute, true, false, "./testdata/agent.yaml"))
	require.NoError(t, kubeapply.Kubeapply(ctx, kubeinfo, 2*time.Minute, true, false, "./testdata/fake-agentcom.yaml"))

	time.Sleep(3 * time.Second)
}
