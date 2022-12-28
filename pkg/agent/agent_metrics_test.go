package agent

import (
	"net"
	"testing"
	"time"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/peer"

	"github.com/datawire/ambassador-agent/pkg/api/agent"
	"github.com/datawire/dlib/dlog"
	envoyMetrics "github.com/emissary-ingress/emissary/v3/pkg/api/envoy/service/metrics/v3"
)

var (
	counterType    = io_prometheus_client.MetricType_COUNTER
	acceptedMetric = &io_prometheus_client.MetricFamily{
		Name: StrToPointer("cluster.apple_prod_443.upstream_rq_total"),
		Type: &counterType,
		Metric: []*io_prometheus_client.Metric{
			{
				Counter: &io_prometheus_client.Counter{
					Value: Float64ToPointer(42),
				},
				TimestampMs: Int64ToPointer(time.Now().Unix() * 1000),
			},
		},
	}
	ignoredMetric = &io_prometheus_client.MetricFamily{
		Name: StrToPointer("cluster.apple_prod_443.metric_to_ignore"),
		Type: &counterType,
		Metric: []*io_prometheus_client.Metric{
			{
				Counter: &io_prometheus_client.Counter{
					Value: Float64ToPointer(42),
				},
				TimestampMs: Int64ToPointer(time.Now().Unix() * 1000),
			},
		},
	}
)

func agentMetricsSetupTest() (*MockClient, *Agent) {
	clientMock := &MockClient{}

	stubbedAgent := &Agent{
		comm: &RPCComm{
			client: clientMock,
		},
		aggregatedMetrics:     map[string][]*io_prometheus_client.MetricFamily{},
		metricsReportComplete: make(chan error),
	}

	return clientMock, stubbedAgent
}

func TestMetricsRelayHandler(t *testing.T) {
	t.Run("will relay metrics from the stack", func(t *testing.T) {
		// given
		clientMock, stubbedAgent := agentMetricsSetupTest()
		ctx := peer.NewContext(dlog.NewTestContext(t, true), &peer.Peer{
			Addr: &net.IPAddr{
				IP: net.ParseIP("192.168.0.1"),
			},
		})

		// when
		// store acceptedMetric and reject ignoredMetric
		stubbedAgent.MetricsRelayHandler(ctx, &envoyMetrics.StreamMetricsMessage{
			Identifier:   nil,
			EnvoyMetrics: []*io_prometheus_client.MetricFamily{ignoredMetric, acceptedMetric},
		})
		// report
		stubbedAgent.ReportMetrics(ctx)
		// wait for report to complete
		<-stubbedAgent.metricsReportComplete

		// then
		assert.Equal(t, []*agent.StreamMetricsMessage{{
			EnvoyMetrics: []*io_prometheus_client.MetricFamily{acceptedMetric},
		}}, clientMock.SentMetrics, "metrics should be propagated to cloud")
	})
	t.Run("peer IP is not available", func(t *testing.T) {
		// given
		clientMock, stubbedAgent := agentMetricsSetupTest()
		ctx := dlog.NewTestContext(t, true)

		// when
		stubbedAgent.MetricsRelayHandler(ctx, &envoyMetrics.StreamMetricsMessage{
			Identifier:   nil,
			EnvoyMetrics: []*io_prometheus_client.MetricFamily{acceptedMetric},
		})

		// then
		assert.Equal(t, 0, len(stubbedAgent.aggregatedMetrics), "no metrics")
		assert.Equal(t, 0, len(clientMock.SentMetrics), "nothing send to cloud")
	})
	t.Run("no metrics available in aggregatedMetrics", func(t *testing.T) {
		// given
		clientMock, stubbedAgent := agentMetricsSetupTest()
		ctx := peer.NewContext(dlog.NewTestContext(t, true), &peer.Peer{
			Addr: &net.IPAddr{
				IP: net.ParseIP("192.168.0.1"),
			},
		})

		// when
		stubbedAgent.MetricsRelayHandler(ctx, &envoyMetrics.StreamMetricsMessage{
			Identifier:   nil,
			EnvoyMetrics: []*io_prometheus_client.MetricFamily{},
		})

		// then
		assert.Equal(t, 0, len(stubbedAgent.aggregatedMetrics), "no metrics")
		assert.Equal(t, 0, len(clientMock.SentMetrics), "nothing send to cloud")
	})
}
