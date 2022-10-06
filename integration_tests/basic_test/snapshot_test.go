package basic_test

import (
	"encoding/json"
	"strings"
	"time"

	itest "github.com/datawire/ambassador-agent/integration_tests"
	"github.com/datawire/ambassador-agent/pkg/api/agent"
	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func (s *BasicTestSuite) TestEmptySnapshot() {
	var (
		snapshotBytes []byte
		timer         = time.NewTimer(5 * time.Second)
	)

	for snapshotBytes == nil {
		select {
		case <-timer.C:
			s.FailNow("timed out waiting for snapshot")
		default:
			outBytes, _, err := itest.Exec(s.config, s.namespace, "agentcom-server", "cat", "/tmp/snapshot.json")
			if err != nil {
				if strings.Contains(err.Error(), "exit code 1") {
					err = nil
				}
			}
			s.Require().NoError(err)
			if 0 < len(outBytes) {
				snapshotBytes = outBytes
			} else {
				time.Sleep(time.Second)
			}
		}
	}

	s.NotEmpty(snapshotBytes)

	var (
		reportSnapshot agent.Snapshot
		snapshot       snapshotTypes.Snapshot
	)

	err := json.Unmarshal(snapshotBytes, &reportSnapshot)
	s.Require().NoError(err)

	err = json.Unmarshal(reportSnapshot.RawSnapshot, &snapshot)
	s.Require().NoError(err)

	s.Empty(snapshot.Deltas)
}
