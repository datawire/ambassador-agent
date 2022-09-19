package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/datawire/dlib/dlog"
	"github.com/google/uuid"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetClusterID(ctx context.Context, client *kubernetes.Clientset) (clusterID string) {
	clusterID = getEnvWithDefault("AMBASSADOR_CLUSTER_ID", getEnvWithDefault("AMBASSADOR_SCOUT_ID", ""))
	if clusterID != "" {
		return clusterID
	}

	nsName := "default"
	/*
		// TODO scoped agent logic
		if IsAmbassadorSingleNamespace() {
			nsName = GetAmbassadorNamespace()
		}
	*/

	dlog.Infof(ctx, "Fetching cluster ID from %s namespace", nsName)

	ns, err := client.CoreV1().Namespaces().Get(ctx, nsName, v1.GetOptions{})
	if err == nil {
		clusterID = string(ns.GetUID())
	} else {
		dlog.Errorf(ctx, "Unable to detect cluster ID: %v", err)
		return ""
	}

	dlog.Infof(ctx, "Cluster ID is %s", clusterID)
	dlog.Debugf(ctx, "Namespace looks like %+v", ns)
	return clusterIDFromRootID(clusterID)
}

func clusterIDFromRootID(rootID string) string {
	clusterUrl := fmt.Sprintf("d6e_id://%s/00000000-0000-0000-0000-000000000000", rootID)
	uid := uuid.NewSHA1(uuid.NameSpaceURL, []byte(clusterUrl))
	return strings.ToLower(uid.String())
}
