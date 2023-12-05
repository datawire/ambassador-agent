package itest

import (
	"context"
	"encoding/json"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/datawire/ambassador-agent/pkg/api/agent"
	"github.com/datawire/dlib/dlog"
	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
)

func (s *BasicTestSuite) TestInitialSnapshot() {
	ctx := s.Context()
	var ss *agent.Snapshot
	s.Require().Eventually(func() bool {
		var err error
		ss, err = s.agentComServer.GetSnapshot(ctx)
		if err != nil {
			dlog.Errorf(ctx, "GetSnapshot failed: %v", err)
		}
		return err == nil
	}, 15*time.Second, 2*time.Second)

	var snapshot snapshotTypes.Snapshot
	s.Require().NoError(json.Unmarshal(ss.RawSnapshot, &snapshot))

	s.Empty(snapshot.Deltas)
}

func (s *BasicTestSuite) TestSnapshot() {
	ctx, cancel := context.WithTimeout(s.Context(), 15*time.Second)
	defer cancel()
	ss, err := s.agentComServer.GetSnapshot(ctx)
	s.Require().NoError(err)

	var snapshot snapshotTypes.Snapshot
	err = json.Unmarshal(ss.RawSnapshot, &snapshot)
	s.Require().NoError(err)

	for _, pod := range snapshot.Kubernetes.Pods {
		s.Equal(pod.APIVersion, "v1")
		s.Equal(pod.Kind, "Pod")
		s.Empty(pod.ObjectMeta.ManagedFields)
	}

	for _, dep := range snapshot.Kubernetes.Endpoints {
		s.Equal(dep.APIVersion, "v1")
		s.Equal(dep.Kind, "Endpoint")
		s.Empty(dep.ObjectMeta.ManagedFields)
	}

	for _, svc := range snapshot.Kubernetes.Services {
		s.Equal(svc.APIVersion, "v1")
		s.Equal(svc.Kind, "Service")
		s.Empty(svc.ObjectMeta.ManagedFields)
	}

	for _, svc := range snapshot.Kubernetes.ConfigMaps {
		s.Equal(svc.APIVersion, "v1")
		s.Equal(svc.Kind, "ConfigMap")
		s.Empty(svc.ObjectMeta.ManagedFields)
	}

	for _, svc := range snapshot.Kubernetes.Ingresses {
		s.Equal(svc.APIVersion, "networking.kubernetes.io/v1")
		s.Equal(svc.Kind, "Ingress")
		s.Empty(svc.ObjectMeta.ManagedFields)
	}

	for _, dep := range snapshot.Kubernetes.Deployments {
		s.Equal(dep.APIVersion, "apps/v1")
		s.Equal(dep.Kind, "Deployment")
		s.Empty(dep.ObjectMeta.ManagedFields)
	}
}
