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

	rootID := "00000000-0000-0000-0000-000000000000"

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
		rootID = string(ns.GetUID())
	}

	dlog.Infof(ctx, "Cluster ID is %s", clusterID)
	return clusterIDFromRootID(rootID)
}

func clusterIDFromRootID(rootID string) string {
	clusterUrl := fmt.Sprintf("d6e_id://%s/00000000-0000-0000-0000-000000000000", rootID)
	uid := uuid.NewSHA1(uuid.NameSpaceURL, []byte(clusterUrl))
	return strings.ToLower(uid.String())
}
