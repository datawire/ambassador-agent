package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"github.com/google/uuid"
	"k8s.io/client-go/rest"
)

func GetClusterID(ctx context.Context, client rest.Interface) (clusterID string) {
	clusterID = getEnvWithDefault("AMBASSADOR_CLUSTER_ID", getEnvWithDefault("AMBASSADOR_SCOUT_ID", ""))
	if clusterID != "" {
		return clusterID
	}

	rootID := "00000000-0000-0000-0000-000000000000"

	client, err := kates.NewClient(kates.ClientConfig{})
	if err == nil {
		nsName := "default"
		if IsAmbassadorSingleNamespace() {
			nsName = GetAmbassadorNamespace()
		}
		ns := &kates.Namespace{
			TypeMeta:   kates.TypeMeta{Kind: "Namespace"},
			ObjectMeta: kates.ObjectMeta{Name: nsName},
		}

		err := client.Get(ctx, ns, ns)
		if err == nil {
			rootID = string(ns.GetUID())
		}
	}

	return clusterIDFromRootID(rootID)
}

func clusterIDFromRootID(rootID string) string {
	clusterUrl := fmt.Sprintf("d6e_id://%s/%s", rootID, GetAmbassadorID())
	uid := uuid.NewSHA1(uuid.NameSpaceURL, []byte(clusterUrl))

	return strings.ToLower(uid.String())
}
