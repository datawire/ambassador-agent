package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/datawire/dlib/dlog"
	"github.com/google/uuid"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetClusterID(ctx context.Context, client *kubernetes.Clientset, nsName string) string {
	clusterID := getEnvWithDefault("AMBASSADOR_CLUSTER_ID", getEnvWithDefault("AMBASSADOR_SCOUT_ID", ""))
	if clusterID != "" {
		return clusterID
	}

	dlog.Infof(ctx, "Fetching cluster ID from %s namespace", nsName)

	rootID := "00000000-0000-0000-0000-000000000000"

	ns, err := client.CoreV1().Namespaces().Get(ctx, nsName, v1.GetOptions{})
	if err == nil {
		rootID = string(ns.GetUID())
	}

	dlog.Infof(ctx, "Using root ID %s", rootID)
	dlog.Debugf(ctx, "Namespace looks like %+v", ns)
	return clusterIDFromRootID(clusterID)
}

func clusterIDFromRootID(rootID string) string {
	clusterUrl := fmt.Sprintf("d6e_id://%s/%s", rootID, getAmbassadorID())
	uid := uuid.NewSHA1(uuid.NameSpaceURL, []byte(clusterUrl))

	return strings.ToLower(uid.String())
}

func getAmbassadorID() string {
	id := os.Getenv("AMBASSADOR_ID")
	if id == "" {
		id = "default"
	}
	return id
}
