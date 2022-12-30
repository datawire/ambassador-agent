package agent

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/datawire/ambassador-agent/rpc/agent"
	"github.com/datawire/dlib/dhttp"
	"github.com/datawire/dlib/dlog"
	emissaryApi "github.com/emissary-ingress/emissary/v3/pkg/api/getambassador.io/v3alpha1"
	"github.com/emissary-ingress/emissary/v3/pkg/snapshot/v1"
)

func (a *Agent) Service(ctx context.Context) error {
	svr := grpc.NewServer()
	agent.RegisterAgentServer(svr, a)
	sc := &dhttp.ServerConfig{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			svr.ServeHTTP(w, r)
		}),
	}
	return sc.ListenAndServe(ctx, net.JoinHostPort(a.ServerHost, strconv.Itoa(int(a.ServerPort))))
}

func (a *Agent) ResolveIngress(ctx context.Context, request *agent.IngressInfoRequest) (*agent.IngressInfoResponse, error) {
	r, msg := a.getSnapshotIngress(request)
	if r != nil {
		return r, nil
	}
	dlog.Debugf(ctx, "%s, returning default ingress info", msg)
	return a.getDefaultIngressInfo(request), nil
}

func (a *Agent) Version(context.Context, *emptypb.Empty) (*agent.VersionInfo, error) {
	return &agent.VersionInfo{
		Name:    "ambassador-agent",
		Version: Version,
	}, nil
}

func (a *Agent) getDefaultIngressInfo(request *agent.IngressInfoRequest) *agent.IngressInfoResponse {
	fqdn := fmt.Sprintf("%s.%s.svc.%s", request.ServiceName, request.Namespace, a.clusterDomain)
	return &agent.IngressInfoResponse{
		L3Host: fqdn,
		Port:   request.ServicePortNumber,
		L5Host: fqdn,
		UseTls: request.ServicePortName == "https" || request.ServicePortNumber == 443,
	}
}

func (a *Agent) getSnapshotIngress(request *agent.IngressInfoRequest) (*agent.IngressInfoResponse, string) {
	a.currentSnapshotMutex.Lock()
	sn := a.currentSnapshot
	a.currentSnapshotMutex.Unlock()
	if sn == nil {
		return nil, "No current snapshot"
	}
	ksn := sn.Kubernetes
	if ksn == nil {
		return nil, "No Kubernetes snapshot in current snapshot"
	}
	svc := findServiceInSnapshot(ksn, types.UID(request.ServiceId))
	if svc == nil {
		return nil, fmt.Sprintf("No snapshot found for service %q", request.ServiceId)
	}
	mappings := findServiceMappingsInSnapshot(ksn, svc.Name, svc.Namespace)
	hostName := findHostname(mappings)
	if hostName == "" {
		return nil, fmt.Sprintf("Could not resolve hostname in mappings of service %q", request.ServiceId)
	}
	ingressSvc := findIngressByNameInSnapshot(ksn, "emissary-ingress", "edge-stack", "ambassador")
	if ingressSvc == nil {
		return nil, "No ingress candidate found in cluster"
	}
	response := &agent.IngressInfoResponse{
		L3Host: fmt.Sprintf("%s.%s.svc.%s", ingressSvc.Name, ingressSvc.Namespace, a.clusterDomain),
		L5Host: hostName,
	}
	response.Port, response.UseTls = resolveIngressPort(ingressSvc.Spec.Ports)
	return response, ""
}

func findServiceInSnapshot(snapshot *snapshot.KubernetesSnapshot, serviceID types.UID) *core.Service {
	for _, svc := range snapshot.Services {
		if svc.UID == serviceID {
			return svc
		}
	}
	return nil
}

func findIngressByNameInSnapshot(snapshot *snapshot.KubernetesSnapshot, names ...string) *core.Service {
	for _, name := range names {
		for _, svc := range snapshot.Services {
			if svc.Name == name && len(svc.Spec.Ports) > 0 {
				return svc
			}
		}
	}
	return nil
}

func findServiceMappingsInSnapshot(snapshot *snapshot.KubernetesSnapshot, name, namespace string) []*emissaryApi.Mapping {
	mm := make(map[types.UID]*emissaryApi.Mapping)
	for _, m := range snapshot.Mappings {
		if m.Namespace == namespace {
			// Parse the service. It might be a URL, or simply a hostname.
			svc := m.Spec.Service
			if svc == name {
				mm[m.UID] = m
			} else if parsedURL, err := url.Parse(svc); err == nil && parsedURL.Hostname() == name {
				mm[m.UID] = m
			}
		}
	}
	sm := make([]*emissaryApi.Mapping, len(mm))
	i := 0
	for _, m := range mm {
		sm[i] = m
		i++
	}
	return sm
}

func findHostname(mappings []*emissaryApi.Mapping) string {
	for _, m := range mappings {
		hostname := m.Spec.Hostname
		if hostname != "" && !strings.HasPrefix(hostname, "*") {
			return hostname
		}
	}
	return ""
}

func resolveIngressPort(ports []core.ServicePort) (int32, bool) {
	var httpsPort int32
	var httpPort int32
	foundPorts := make([]int32, 0, len(ports))
	for _, p := range ports {
		port := p.Port
		if port == 443 || p.Name == "https" {
			httpsPort = port
			break
		}
		if port == 80 || p.Name == "http" {
			httpPort = port
		}
		foundPorts = append(foundPorts, port)
	}
	var resolvedPort int32
	var resolvedTLS bool
	switch {
	case httpsPort > 0:
		resolvedPort = httpsPort
		resolvedTLS = true
	case httpPort > 0:
		resolvedPort = httpPort
	default:
		resolvedPort = foundPorts[0]
	}
	return resolvedPort, resolvedTLS
}
