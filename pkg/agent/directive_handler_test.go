package agent_test

import (
	"testing"

	"github.com/datawire/ambassador-agent/pkg/agent"
	agentapi "github.com/datawire/ambassador-agent/pkg/api/agent"
	"github.com/datawire/dlib/dlog"
)

func TestHandleDirective(t *testing.T) {
	ctx := dlog.NewTestContext(t, false)

	a := &agent.Agent{}
	dh := &agent.BasicDirectiveHandler{}

	d := &agentapi.Directive{ID: "one"}

	dh.HandleDirective(ctx, a, d)
}

func TestHandleSecretSyncDirective(t *testing.T) {
	// given
	ctx := dlog.NewTestContext(t, false)

	a := &agent.Agent{}
	dh := &agent.BasicDirectiveHandler{}

	d := &agentapi.Directive{
		ID: "one",
		Commands: []*agentapi.Command{
			{
				SecretSyncCommand: &agentapi.SecretSyncCommand{
					Name:      "my-secret",
					Namespace: "my-namespace",
					CommandId: "1234",
					Action:    agentapi.SecretSyncCommand_SET,
					Secret: map[string][]byte{
						"my-key": []byte("abcd"),
					},
				},
			},
		},
	}

	// when
	dh.HandleDirective(ctx, a, d)
}
