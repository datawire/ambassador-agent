package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/datawire/dlib/dlog"
)

func (a *Agent) getClusterID(ctx context.Context, client *kubernetes.Clientset, nsName string) string {
	clusterID := a.ClusterID
	if clusterID != "" {
		dlog.Infof(ctx, "Using cluster ID from env %s", clusterID)
		return clusterID
	}

	dlog.Infof(ctx, "Fetching cluster ID from namespace %s", nsName)

	rootID := "00000000-0000-0000-0000-000000000000"

	ns, err := client.CoreV1().Namespaces().Get(ctx, nsName, v1.GetOptions{})
	if err == nil {
		rootID = string(ns.GetUID())
	}

	dlog.Infof(ctx, "Using root ID %s", rootID)
	dlog.Debugf(ctx, "Namespace looks like %+v", ns)
	clusterID = a.clusterIDFromRootID(rootID)

	dlog.Infof(ctx, "Using computed cluster ID %s", clusterID)
	return clusterID
}

func (a *Agent) clusterIDFromRootID(rootID string) string {
	clusterUrl := fmt.Sprintf("d6e_id://%s/%s", rootID, a.AmbassadorID)
	uid := uuid.NewSHA1(uuid.NameSpaceURL, []byte(clusterUrl))

	return strings.ToLower(uid.String())
}
