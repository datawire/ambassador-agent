package basic_test

import (
	"context"
	"encoding/json"
	"time"

	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func (s *BasicTestSuite) TestEmptySnapshot() {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	ss, err := s.agentComServer.GetSnapshot(ctx)
	s.Require().NoError(err)

	var snapshot snapshotTypes.Snapshot
	err = json.Unmarshal(ss.RawSnapshot, &snapshot)
	s.Require().NoError(err)

	s.Empty(snapshot.Deltas)
}
