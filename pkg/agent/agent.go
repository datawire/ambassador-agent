package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	diagnosticsTypes "github.com/emissary-ingress/emissary/v3/pkg/diagnostics/v1"
	"github.com/emissary-ingress/emissary/v3/pkg/kates"
	"github.com/emissary-ingress/emissary/v3/pkg/kates/k8s_resource_types"
	snapshotTypes "github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"

	// load all auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/datawire/ambassador-agent/pkg/agent/watchers"
	"github.com/datawire/ambassador-agent/pkg/api/agent"
	rpc "github.com/datawire/ambassador-agent/rpc/agent"
	"github.com/datawire/dlib/dlog"
	"github.com/datawire/k8sapi/pkg/k8sapi"
)

const defaultMinReportPeriod = 30 * time.Second
const (
	cloudConnectTokenKey = "CLOUD_CONNECT_TOKEN"
)

type Comm interface {
	Close() error
	Report(context.Context, *agent.Snapshot, string) error
	ReportCommandResult(context.Context, *agent.CommandResult, string) error
	Directives() <-chan *agent.Directive
	StreamMetrics(context.Context, *agent.StreamMetricsMessage, string) error
	StreamDiagnostics(context.Context, *agent.Diagnostics, string) error
}

// Agent is the component that talks to the DCP Director, which is a cloud
// service run by Datawire. It is also gRPC AgentServer in itself.
type Agent struct {
	rpc.UnsafeAgentServer
	*Env
	comm             Comm
	agentID          *agent.Identity
	newDirective     <-chan *agent.Directive
	directiveHandler DirectiveHandler
	// store what the initial value was in the env var, so we can set the ambassadorAPIKey value
	// (^^Above) if the configmap and/or secret get deleted.
	ambassadorAPIKeyEnvVarValue string

	// State managed by the director via the retriever
	reportingStopped bool // Did the director say don't report?
	lastDirectiveID  string

	// The state of reporting
	reportToSend   *agent.Snapshot // Report that's ready to send
	reportRunning  atomic.Bool     // Is a report being sent right now?
	reportComplete chan error      // Report() finished with this error

	// apiDocsStore holds OpenAPI documents from cluster Mappings
	apiDocsStore *APIDocsStore

	argoLock sync.Mutex
	// rolloutStore holds Argo Rollouts state from cluster
	rolloutStore *RolloutStore
	// applicationStore holds Argo Applications state from cluster
	applicationStore *ApplicationStore

	// Extra headers to inject into RPC requests to ambassador cloud.
	rpcExtraHeaders []string

	// Diagnostics reporting
	reportDiagnosticsAllowed    bool // Allow agent to fetch diagnostics and report to cloud
	diagnosticsReportingStopped bool // Director stopped diagnostics reporting
	// minDiagnosticsReportPeriod  time.Duration // How frequently do we collect diagnostics

	// The state of diagnostic reporting
	diagnosticsReportRunning  atomic.Bool // Is a report being sent right now?
	diagnosticsReportComplete chan error  // Report() finished with this error

	// Stand-alone config
	emissaryPresent bool   // if not installed by emissary, generate snapshots
	clusterId       string // cluster id used in generated snapshots
	clusterDomain   string // the cluster domain name, e.g. .cluster.local

	// snapshot watchers
	coreWatchers    watchers.SnapshotWatcher
	fallbackWatcher watchers.SnapshotWatcher
	// config watchers
	configWatchers    *ConfigWatchers
	ambassadorWatcher *AmbassadorWatcher

	currentSnapshotMutex sync.Mutex
	currentSnapshot      *snapshotTypes.Snapshot
}

// NewAgent returns a new Agent.
func NewAgent(
	ctx context.Context,
	directiveHandler DirectiveHandler,
	rolloutsGetterFactory rolloutsGetterFactory,
	secretsGetterFactory secretsGetterFactory,
	env *Env,
) *Agent {
	if directiveHandler == nil {
		directiveHandler = &BasicDirectiveHandler{
			DefaultMinReportPeriod: defaultMinReportPeriod,
			rolloutsGetterFactory:  rolloutsGetterFactory,
			secretsGetterFactory:   secretsGetterFactory,
		}
	}

	rpcExtraHeaders := make([]string, 0)

	if env.RpcInterceptHeaderKey != "" && env.RpcInterceptHeaderValue != "" {
		rpcExtraHeaders = append(
			rpcExtraHeaders,
			env.RpcInterceptHeaderKey,
			env.RpcInterceptHeaderValue,
		)
	}

	apiSvc := "kubernetes.default"
	var clusterDomain string
	if cn, err := net.LookupCNAME(apiSvc); err != nil {
		dlog.Infof(ctx, `Unable to determine cluster domain from CNAME of %s: %v"`, err, apiSvc)
		clusterDomain = "cluster.local"
	} else {
		clusterDomain = cn[len(apiSvc)+5 : len(cn)-1] // Strip off "kubernetes.default.svc." and trailing dot
	}
	dlog.Infof(ctx, "Using cluster domain %q", clusterDomain)

	return &Agent{
		Env:            env,
		reportComplete: make(chan error),

		ambassadorAPIKeyEnvVarValue: env.AmbassadorAPIKey,
		directiveHandler:            directiveHandler,
		rpcExtraHeaders:             rpcExtraHeaders,

		// k8sapi watchers
		coreWatchers:      watchers.NewCoreWatchers(ctx, env.NamespacesToWatch, objectModifier),
		configWatchers:    NewConfigWatchers(ctx, env.AgentNamespace),
		ambassadorWatcher: NewAmbassadorWatcher(ctx, env.AgentNamespace),
		fallbackWatcher:   watchers.NewFallbackWatcher(ctx, env.NamespacesToWatch, objectModifier),
		clusterDomain:     clusterDomain,
	}
}

func (a *Agent) StopReporting(ctx context.Context) {
	dlog.Debugf(ctx, "stop reporting: %t -> true", a.reportingStopped)
	a.reportingStopped = true
}

func (a *Agent) StartReporting(ctx context.Context) {
	dlog.Debugf(ctx, "stop reporting: %t -> false", a.reportingStopped)
	a.reportingStopped = false
}

func (a *Agent) SetMinReportPeriod(ctx context.Context, dur time.Duration) {
	dlog.Debugf(ctx, "minimum report period %s -> %s", a.MinReportPeriod, dur)
	a.MinReportPeriod = dur
}

func (a *Agent) SetLastDirectiveID(ctx context.Context, id string) {
	dlog.Debugf(ctx, "setting last directive ID %s", id)
	a.lastDirectiveID = id
}

func (a *Agent) SetReportDiagnosticsAllowed(reportDiagnosticsAllowed bool) {
	dlog.Debugf(context.Background(), "setting reporting diagnostics to cloud to: %t", reportDiagnosticsAllowed)
	a.reportDiagnosticsAllowed = reportDiagnosticsAllowed
}

func getAmbSnapshotInfo(url *url.URL) (*snapshotTypes.Snapshot, error) {
	// TODO maybe put request in go-routine
	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		return nil, errors.New(fmt.Sprintf("Cannot fetch snapshot from url: %s. "+
			"Response failed with status code: %d", url, resp.StatusCode))
	}
	defer resp.Body.Close()
	rawSnapshot, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ret := &snapshotTypes.Snapshot{}
	err = json.Unmarshal(rawSnapshot, ret)

	return ret, err
}

func getAmbDiagnosticsInfo(url *url.URL) (*diagnosticsTypes.Diagnostics, error) {
	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 {
		return nil, errors.New(fmt.Sprintf("Cannot fetch diagnostics from url: %s. "+
			"Response failed with status code: %d", url, resp.StatusCode))
	}
	defer resp.Body.Close()
	rawDiagnosticsSnapshot, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ret := &diagnosticsTypes.Diagnostics{}
	err = json.Unmarshal(rawDiagnosticsSnapshot, ret)

	return ret, err
}

func (a *Agent) handleAPIKeyConfigChange(ctx context.Context) {
	secret := &kates.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      a.AgentConfigResourceName,
		Namespace: a.AgentNamespace,
	}}

	var (
		ok  bool
		err error
	)

	maybeResetComm := func(newKey string) {
		if newKey != a.AmbassadorAPIKey {
			a.AmbassadorAPIKey = newKey
			a.ClearComm()
		}
	}

	secret, ok, err = a.configWatchers.secretWatcher.Get(ctx, secret)
	if err != nil {
		dlog.Warnf(ctx, "unable to retrieve cloud connect token secret: %v", err)
	}

	if !ok {
		cm := &kates.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      a.AgentConfigResourceName,
			Namespace: a.AgentNamespace,
		}}
		cm, ok, err = a.configWatchers.mapsWatcher.Get(ctx, cm)
		if err != nil {
			dlog.Warnf(ctx, "unable to retrieve cloud connect token configmap: %v", err)
		}
		if !ok {
			// so if we got here, we know something changed, but a config map
			// nor a secret exist, which means they never existed, or they got
			// deleted. in this case, we fall back to the env var (which is
			// likely empty, so in that case, that is basically equivalent to
			// turning the agent "off")
			dlog.Infof(ctx, "Setting cloud connect token from environment")
			if a.ambassadorAPIKeyEnvVarValue == "" {
				dlog.Errorf(ctx, "Unable to get cloud connect token. This agent will do nothing.")
			}
			// always run maybeResetComm so that the agent can be turned "off"
			maybeResetComm(a.ambassadorAPIKeyEnvVarValue)
			return
		}

		connTokenBytes := cm.Data[cloudConnectTokenKey]

		dlog.Infof(ctx, "Setting cloud connect token from configmap: %s", cm.GetName())
		maybeResetComm(connTokenBytes)
		return
	}

	connTokenBytes := secret.Data[cloudConnectTokenKey]
	dlog.Infof(ctx, "Setting cloud connect token from secret: %s", secret.GetName())
	maybeResetComm(string(connTokenBytes))
}

// Watch is the work performed by the main goroutine for the Agent. It processes
// Watt/Diag snapshots, reports to the Director, and executes directives from
// the Director.
func (a *Agent) Watch(ctx context.Context) error {
	dlog.Info(ctx, "Agent is running...")
	configCh := k8sapi.Subscribe(ctx, a.configWatchers.cond)
	a.waitForAPIKey(ctx, configCh)
	a.coreWatchers.EnsureStarted(ctx)
	a.handleAmbassadorEndpointChange(ctx, a.AESSnapshotURL.Hostname())
	ambCh := k8sapi.Subscribe(ctx, a.ambassadorWatcher.cond)

	go a.argoWatch(ctx)
	return a.watch(ctx, configCh, ambCh)
}

func hasResource(ctx context.Context, resourceLists []*metav1.APIResourceList, r *schema.GroupVersionResource) bool {
	for _, rl := range resourceLists {
		if r.GroupVersion().String() == rl.GroupVersion {
			for _, ar := range rl.APIResources {
				if r.Resource == ar.Name {
					dlog.Infof(ctx, "Watching %s", r)
					return true
				}
			}
		}
	}
	dlog.Infof(ctx, "Will not watch %s because that resource is not known to this cluster", r)
	return false
}

func (a *Agent) argoWatch(ctx context.Context) {
	client, err := kates.NewClient(kates.ClientConfig{})
	if err != nil {
		dlog.Errorf(ctx, "Error making kates client: %s", err)
		return
	}
	ns := kates.NamespaceAll
	dc := NewDynamicClient(client.DynamicInterface(), NewK8sInformer)

	var cancelRolloutWatch, cancelApplicationWatch context.CancelFunc
	defer func() {
		if cancelRolloutWatch != nil {
			cancelRolloutWatch()
		}
		if cancelApplicationWatch != nil {
			cancelApplicationWatch()
		}
	}()

	for {
		_, resourcesLists, err := k8sapi.GetK8sInterface(ctx).Discovery().ServerGroupsAndResources()
		if err != nil {
			dlog.Errorf(ctx, "Error getting resources list: %s", err)
			return
		}

		// Using a func() here to prevent lint from complaining about a non-existent context leak.
		rolloutGvr, _ := schema.ParseResourceArg("rollouts.v1alpha1.argoproj.io")
		if hasResource(ctx, resourcesLists, rolloutGvr) {
			if cancelRolloutWatch == nil {
				var cctx context.Context
				cctx, cancelRolloutWatch = context.WithCancel(ctx)
				go a.argoRolloutWatch(cctx, dc, ns, rolloutGvr)
			}
		} else {
			if cancelRolloutWatch != nil {
				cancelRolloutWatch()
				cancelRolloutWatch = nil
			}
		}

		applicationGvr, _ := schema.ParseResourceArg("applications.v1alpha1.argoproj.io")
		if hasResource(ctx, resourcesLists, applicationGvr) {
			if cancelApplicationWatch == nil {
				var cctx context.Context
				cctx, cancelApplicationWatch = context.WithCancel(ctx)
				go a.argoApplicationWatch(cctx, dc, ns, applicationGvr)
			}
		} else {
			if cancelApplicationWatch != nil {
				cancelApplicationWatch()
				cancelApplicationWatch = nil
			}
		}

		// recheck conditions periodically
		recheckDuration := time.Minute
		if cancelRolloutWatch != nil && cancelApplicationWatch != nil {
			// Both resources exist. The recheck becomes a matter of discovering if
			// something is removed. That can be done more seldom.
			recheckDuration *= 30
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(recheckDuration):
		}
	}
}

func (a *Agent) argoRolloutWatch(ctx context.Context, dc *DynamicClient, ns string, rolloutGvr *schema.GroupVersionResource) {
	dlog.Infof(ctx, "Watching %s", rolloutGvr)
	rolloutCallbackCh := dc.WatchGeneric(ctx, ns, rolloutGvr)
	rolloutStore := NewRolloutStore()
	for {
		// Wait for an event
		select {
		case <-ctx.Done():
			return
		case callback, ok := <-rolloutCallbackCh:
			if ok {
				dlog.Debugf(ctx, "argo rollout callback: %v", callback.EventType)
				store, err := rolloutStore.FromCallback(callback)
				if err != nil {
					dlog.Warnf(ctx, "Error processing rollout callback: %s", err)
				}
				a.argoLock.Lock()
				a.rolloutStore = store
				a.argoLock.Unlock()
			}
		}
	}
}

func (a *Agent) argoApplicationWatch(ctx context.Context, dc *DynamicClient, ns string, applicationGvr *schema.GroupVersionResource) {
	dlog.Infof(ctx, "Watching %s", applicationGvr)
	applicationCallbackCh := dc.WatchGeneric(ctx, ns, applicationGvr)
	applicationStore := NewApplicationStore()
	for {
		// Wait for an event
		select {
		case <-ctx.Done():
			return
		case callback, ok := <-applicationCallbackCh:
			if ok {
				dlog.Debugf(ctx, "argo application callback: %v", callback.EventType)
				store, err := applicationStore.FromCallback(callback)
				if err != nil {
					dlog.Warnf(ctx, "Error processing application callback: %s", err)
				}
				a.argoLock.Lock()
				a.applicationStore = store
				a.argoLock.Unlock()
			}
		}
	}
}

func (a *Agent) waitForAPIKey(ctx context.Context, ch <-chan struct{}) {
	a.handleAPIKeyConfigChange(ctx)

	// wait until the user installs an api key
	for a.AmbassadorAPIKey == "" {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			a.handleAPIKeyConfigChange(ctx)
		case <-time.After(1 * time.Minute):
			dlog.Debugf(ctx, "Still waiting for api key")
		}
	}
}

// watch is a synchronous function.
// It uses channels to watch for config changes, if none are firing,
// a report is maybe sent. Atomic booleans are used to interval reporting.
func (a *Agent) watch( //nolint:gocognit,cyclop // TODO: Refactor this function
	ctx context.Context,
	configCh, ambCh <-chan struct{},
) error {
	var err error
	a.apiDocsStore = NewAPIDocsStore()

	dlog.Info(ctx, "Beginning to watch and report resources to ambassador cloud")
	for {
		// Wait for an event
		select {
		case <-ctx.Done():
			return nil
			// just hardcode it so we wake every 1 second and check if we're ready to report
			// intentionally not waiting for agent.minReportPeriod seconds because then we may
			// never report if a bunch of directives keep coming in or pods change a
			// bunch
		case <-time.After(1 * time.Second):
			// just a ticker, this will fallthrough to the snapshot getting thing
		case <-configCh:
			a.handleAPIKeyConfigChange(ctx)
		case <-ambCh:
			a.handleAmbassadorEndpointChange(ctx, a.AESSnapshotURL.Hostname())
		case directive := <-a.newDirective:
			a.directiveHandler.HandleDirective(ctx, a, directive)
		}

		// only ask ambassador for a snapshot if we're actually going to report it.
		// if reportRunning is true, that means we're still in the quiet period
		// after sending a report.
		// if emissary is the owner, do all the things
		if !a.reportingStopped && !a.reportRunning.Load() {
			// if emissary is present, get initial snapshot from emissary
			// otherwise, create it
			var snapshot *snapshotTypes.Snapshot
			if a.emissaryPresent {
				snapshot, err = getAmbSnapshotInfo(a.AESSnapshotURL)
				if err != nil {
					dlog.Warnf(ctx, "Error getting snapshot from ambassador %+v", err)
				}
			} else {
				a.clusterId = a.getClusterID(ctx, a.AgentNamespace) // get cluster id for ambMeta
				snapshot = &snapshotTypes.Snapshot{
					AmbassadorMeta: &snapshotTypes.AmbassadorMetaInfo{
						ClusterID: a.clusterId,
					},
					Kubernetes: &snapshotTypes.KubernetesSnapshot{},
				}
			}

			dlog.Debug(ctx, "Received snapshot in agent")
			if err = a.ProcessSnapshot(ctx, snapshot); err != nil {
				dlog.Warnf(ctx, "error processing snapshot: %+v", err)
			}
		}

		// We are about to start sending reports. Let's make sure we have a comm and apikey first
		if a.AmbassadorAPIKey == "" {
			dlog.Debugf(ctx, "CLOUD_CONNECT_TOKEN not set in the environment, not reporting diagnostics")
			continue
		}

		// Ensure comm so we can send reports. There is no call to ClearComm until the next loop
		if a.comm == nil {
			// The communications channel to the DCP was not yet created or was
			// closed above, due to a change in identity, or close elsewhere, due to
			// a change in endpoint configuration.
			newComm, err := NewComm(
				ctx, a.ConnAddress, a.agentID, a.AmbassadorAPIKey, a.rpcExtraHeaders)
			if err != nil {
				dlog.Warnf(ctx, "Failed to dial the DCP: %v", err)
				dlog.Warn(ctx, "DCP functionality disabled until next retry")
				continue
			}

			a.comm = newComm
			a.newDirective = a.comm.Directives()
		}

		if !a.reportingStopped && !a.reportRunning.Load() && a.reportToSend != nil {
			a.ReportSnapshot(ctx)
		} else {
			// Don't report if the Director told us to stop reporting, if we are
			// already sending a report or waiting for the minimum time between
			// reports, or if there is nothing new to report right now.
			dlog.Tracef(ctx, "Not reporting snapshot [reporting stopped = %t] [report running = %t] [report to send is nil = %t]",
				a.reportingStopped, a.reportRunning.Load(), a.reportToSend == nil)
		}

		// only get diagnostics from edgissary if it is present
		// TODO get metrics/diagnostics from traffic manager?
		if !a.emissaryPresent {
			dlog.Tracef(ctx, "Edgissary not present, not reporting edgissary diagnostics")
			continue
		}

		if !a.diagnosticsReportingStopped && !a.diagnosticsReportRunning.Load() && a.reportDiagnosticsAllowed {
			a.ReportDiagnostics(ctx, a.AESDiagnosticsURL)
		} else {
			// Don't report if the Director told us to stop reporting, if we are
			// already sending a report or waiting for the minimum time between
			// reports
			dlog.Tracef(ctx, "Not reporting diagnostics [reporting stopped = %t] [report running = %t]", a.diagnosticsReportingStopped, a.diagnosticsReportRunning.Load())
		}
	}
}

func (a *Agent) handleAmbassadorEndpointChange(ctx context.Context, ambassadorHost string) {
	target := strings.Split(ambassadorHost, ".")[0]
	if endpoints, err := a.ambassadorWatcher.endpointWatcher.List(ctx); err == nil {
		for _, endpoint := range endpoints {
			if endpoint.Name == target {
				dlog.Infof(ctx, "%s detected, using emissary snapshots.", target)
				a.emissaryPresent = true
				a.fallbackWatcher.Cancel()
				return
			}
		}
	} else {
		dlog.Warnf(ctx, "Unable to watch for ambassador-admin service, will act as though standalone: %v", err)
	}
	dlog.Infof(ctx, "%s not detected, creating own snapshots.", target)
	a.emissaryPresent = false
	a.fallbackWatcher.EnsureStarted(ctx)
}

func (a *Agent) ReportSnapshot(ctx context.Context) {
	dlog.Debugf(ctx, "Sending snapshot")
	a.reportRunning.Store(true) // Cleared when the report completes

	// Send a report. This is an RPC, i.e. it can block, so we do this in a
	// goroutine. Sleep after send, so we don't need to keep track of
	// whether/when it's okay to send the next report.
	go func(ctx context.Context, report *agent.Snapshot, delay time.Duration, apikey string) {
		err := a.comm.Report(ctx, report, apikey)
		if err != nil {
			dlog.Warnf(ctx, "failed to report: %+v", err)
		}
		dlog.Debugf(ctx, "Finished sending snapshot report, sleeping for %s", delay.String())
		time.Sleep(delay)
		a.reportRunning.Store(false)

		// make write non-blocking
		select {
		case a.reportComplete <- err:
			// cool we sent something
		default:
			// do nothing if nobody is listening
		}
	}(ctx, a.reportToSend, a.MinReportPeriod, a.AmbassadorAPIKey)

	// Update state variables
	a.reportToSend = nil // Set when a snapshot yields a fresh report
}

// ReportDiagnostics ...
func (a *Agent) ReportDiagnostics(ctx context.Context, diagnosticsURL *url.URL) {
	// TODO maybe put request in go-routine
	diagnostics, err := getAmbDiagnosticsInfo(diagnosticsURL)
	if err != nil {
		dlog.Warnf(ctx, "Error getting diagnostics from ambassador %+v", err)
	}
	dlog.Debug(ctx, "Received diagnostics in agent")
	agentDiagnostics, err := a.ProcessDiagnostics(ctx, diagnostics)
	if err != nil {
		dlog.Warnf(ctx, "error processing diagnostics: %+v", err)
	}
	if agentDiagnostics == nil {
		dlog.Debug(ctx, "No diagnostics exist post-processing, not reporting diagnostics")
		return
	}

	a.diagnosticsReportRunning.Store(true) // Cleared when the diagnostics report completes

	// Send a diagnostics report. This is an RPC, i.e. it can block, so we do this in a
	// goroutine. Sleep after send, so we don't need to keep track of
	// whether/when it's okay to send the next report.
	go func(ctx context.Context, diagnosticsReport *agent.Diagnostics, delay time.Duration, apikey string) {
		err := a.comm.StreamDiagnostics(ctx, diagnosticsReport, apikey)
		if err != nil {
			dlog.Warnf(ctx, "failed to do diagnostics report: %+v", err)
		}
		dlog.Debugf(ctx, "Finished sending diagnostics report, sleeping for %s", delay.String())
		time.Sleep(delay)
		a.diagnosticsReportRunning.Store(false)

		// make write non-blocking
		select {
		case a.diagnosticsReportComplete <- err:
			// cool we sent something
		default:
			// do nothing if nobody is listening
		}
	}(ctx, agentDiagnostics, a.MinReportPeriod, a.AmbassadorAPIKey) // minReportPeriod is the one set for snapshots
}

// ProcessSnapshot turns a Watt/Diag Snapshot into a report that the agent can
// send to the Director. If the new report is semantically different from the
// prior one sent, then the Agent's state is updated to indicate that reporting
// should occur once again.
func (a *Agent) ProcessSnapshot(ctx context.Context, snapshot *snapshotTypes.Snapshot) error {
	if snapshot == nil || snapshot.AmbassadorMeta == nil {
		dlog.Warn(ctx, "No metadata discovered for snapshot, not reporting.")
		return nil
	}

	agentID := GetIdentity(snapshot.AmbassadorMeta, a.AESSnapshotURL.Hostname())
	if agentID == nil {
		dlog.Warnf(ctx, "Could not parse identity info out of snapshot, not sending snapshot")
		return nil
	}
	a.agentID = agentID

	if snapshot.Kubernetes != nil {
		// load services before pods so that we can do labelMatching
		if !a.emissaryPresent && a.fallbackWatcher != nil {
			a.fallbackWatcher.LoadSnapshot(ctx, snapshot)
		}
		if a.coreWatchers != nil {
			a.coreWatchers.LoadSnapshot(ctx, snapshot)
		}
		a.argoLock.Lock()
		if a.rolloutStore != nil {
			snapshot.Kubernetes.ArgoRollouts = a.rolloutStore.StateOfWorld()
			dlog.Debugf(ctx, "Found %d argo rollouts", len(snapshot.Kubernetes.ArgoRollouts))
		}
		if a.applicationStore != nil {
			snapshot.Kubernetes.ArgoApplications = a.applicationStore.StateOfWorld()
			dlog.Debugf(ctx, "Found %d argo applications", len(snapshot.Kubernetes.ArgoApplications))
		}
		a.argoLock.Unlock()
		if a.apiDocsStore != nil {
			a.apiDocsStore.ProcessSnapshot(ctx, snapshot)
			snapshot.APIDocs = a.apiDocsStore.StateOfWorld()
			dlog.Debugf(ctx, "Found %d api docs", len(snapshot.APIDocs))
		}
	}

	if err := snapshot.Sanitize(); err != nil {
		dlog.Errorf(ctx, "Error sanitizing snapshot: %v", err)
		return err
	}
	a.currentSnapshotMutex.Lock()
	a.currentSnapshot = snapshot
	a.currentSnapshotMutex.Unlock()

	rawJsonSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		dlog.Errorf(ctx, "Error marshalling snapshot: %v", err)
		return err
	}

	report := &agent.Snapshot{
		Identity:    agentID,
		RawSnapshot: rawJsonSnapshot,
		ContentType: snapshotTypes.ContentTypeJSON,
		ApiVersion:  snapshotTypes.ApiVersion,
		SnapshotTs:  timestamppb.Now(),
	}

	a.reportToSend = report

	dlog.Debugf(ctx, "Will send a snapshot for %s", agentID)
	return nil
}

// ProcessDiagnostics translates ambassadors diagnostics into streamable agent diagnostics.
func (a *Agent) ProcessDiagnostics(ctx context.Context, diagnostics *diagnosticsTypes.Diagnostics) (*agent.Diagnostics, error) {
	if diagnostics == nil {
		dlog.Warn(ctx, "No diagnostics found, not reporting.")
		return nil, nil
	}

	if diagnostics.System == nil {
		dlog.Warn(ctx, "Missing System information from diagnostics, not reporting.")
		return nil, nil
	}

	agentID := GetIdentityFromDiagnostics(diagnostics.System, a.AESSnapshotURL.Hostname())
	if agentID == nil {
		dlog.Warn(ctx, "Could not parse identity info out of diagnostics, not sending.")
		return nil, nil
	}
	a.agentID = agentID

	rawJsonDiagnostics, err := json.Marshal(diagnostics)
	if err != nil {
		return nil, err
	}

	diagnosticsReport := &agent.Diagnostics{
		Identity:       agentID,
		RawDiagnostics: rawJsonDiagnostics,
		ContentType:    diagnosticsTypes.ContentTypeJSON,
		ApiVersion:     diagnosticsTypes.ApiVersion,
		SnapshotTs:     timestamppb.Now(),
	}

	return diagnosticsReport, nil
}

// ClearComm ends the current connection to the Director, if it exists, thereby
// forcing a new connection to be created when needed.
func (a *Agent) ClearComm() {
	if a.comm != nil {
		a.comm.Close()
		a.comm = nil
	}
}

// MaxDuration returns the greater of two durations.
func MaxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func objectModifier(obj runtime.Object) {
	switch obj := obj.(type) {
	case *corev1.Pod:
		obj.Kind = "Pod"
		obj.APIVersion = "v1"
		obj.ManagedFields = nil

		obj.TypeMeta.APIVersion = obj.APIVersion
		obj.TypeMeta.Kind = obj.Kind

		obj.ObjectMeta.ManagedFields = nil
	case *corev1.Service:
		obj.Kind = "Service"
		obj.APIVersion = "v1"
		obj.ManagedFields = nil

		obj.TypeMeta.APIVersion = obj.APIVersion
		obj.TypeMeta.Kind = obj.Kind

		obj.ObjectMeta.ManagedFields = nil
	case *corev1.ConfigMap:
		obj.Kind = "ConfigMap"
		obj.APIVersion = "v1"
		obj.ManagedFields = nil

		obj.TypeMeta.APIVersion = obj.APIVersion
		obj.TypeMeta.Kind = obj.Kind

		obj.ObjectMeta.ManagedFields = nil
	case *corev1.Endpoints:
		obj.Kind = "Endpoint"
		obj.APIVersion = "v1"
		obj.ManagedFields = nil

		obj.TypeMeta.APIVersion = obj.APIVersion
		obj.TypeMeta.Kind = obj.Kind

		obj.ObjectMeta.ManagedFields = nil
	case *appsv1.Deployment:
		obj.Kind = "Deployment"
		obj.APIVersion = "apps/v1"
		obj.ManagedFields = nil

		obj.TypeMeta.APIVersion = obj.APIVersion
		obj.TypeMeta.Kind = obj.Kind

		obj.ObjectMeta.ManagedFields = nil
	case *k8s_resource_types.Ingress:
		obj.Kind = "Ingress"
		obj.APIVersion = "extensions/v1beta1"
		obj.ManagedFields = nil

		obj.TypeMeta.APIVersion = obj.APIVersion
		obj.TypeMeta.Kind = obj.Kind

		obj.ObjectMeta.ManagedFields = nil
	case *networkingv1.Ingress:
		obj.Kind = "Ingress"
		obj.APIVersion = "networking.k8s.io/v1"
		obj.ManagedFields = nil

		obj.TypeMeta.APIVersion = obj.APIVersion
		obj.TypeMeta.Kind = obj.Kind

		obj.ObjectMeta.ManagedFields = nil
	}
}
