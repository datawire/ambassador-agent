package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

type IngressResolver struct {
	log            logger.Logger
	serviceCatalog ServiceCatalog
}

type mappingV2OrV3Spec struct {
	Host      string `json:"host,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	HostRegex *bool  `json:"host_regex,omitempty"`
}

type mappingV2OrV3 struct {
	Spec mappingV2OrV3Spec `json:"spec"`
}

func NewIngressResolver(log logger.Logger, serviceCatalog ServiceCatalog) *IngressResolver {
	return &IngressResolver{
		log:            log,
		serviceCatalog: serviceCatalog,
	}
}

// ResolveIngressInfo resolves the best ingress settings for a preview-url intercept based on the provided intercept
// spec. The logic applied is as follows:
// 1.0 Check for a snapshot of the service that will be intercepted in the service catalog
// 1.1 If not found, return the default values
//
// 2.0 Find associated mapping hostname
// 2.1 If found, the hostname will be used as L5 host
// 2.2 If not found, it doesn't make sense to route through edgissary, so return the default values
//
// 3.0 Get all services in the service catalog and find an `ambassador`, `emissary-ingress` or `edge-stack` service in the same cluster as the retrieved service in 1
// 3.1 If found, it's name and namespace will be used as L3 host
// 3.2 If not found, return the default values
//
// 4.0 Check for the ports of the found ingress service to try and resolve the right port & tls config
// 4.1 If a "https" port name or 443 port number is found, use that one and enable tls
// 4.2 If a "http" port name or 80 port number is found, use that one and disable tls
// 4.3 If none of those are found, return the first port in the list and disable tls
// 4.4 If there aren't any ports exposed, return the default values
// 4.5 The default values will route to the intercepted service and port directly, with tls if the name of the service port is "https" or its value is 443.
func (ir *IngressResolver) ResolveIngressInfo(ctx context.Context, userIdentity *auth.UserIdentity, request IngressInfoRequest) (*IngressInfoResponse, error) {
	// Try to find a snapshot in the service catalog for the provided service id
	serviceToIntercept, err := ir.serviceCatalog.Get(userIdentity, request.ServiceID)
	if err != nil {
		if errors.Is(err, servicecatalog.ServiceNotFoundErr) {
			ir.log.Debugf("No snapshot found for service %q, returning default ingress info", request.ServiceID)
			return getDefaultIngressInfo(request), nil
		}
		return nil, fmt.Errorf("could not get service to intercept: %w", err)
	}
	// Find a hostname in the service's mappings
	mappingHostname, err := findHostname(serviceToIntercept)
	if err != nil {
		ir.log.Debugf("Could not resolve hostname in mappings of service %q, returning default ingress info.", request.ServiceID)
		return getDefaultIngressInfo(request), nil
	}

	// Try to find an edgissary instance in the service's cluster
	serviceSummaries, err := ir.serviceCatalog.ListV2(userIdentity, &servicecatalog.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not list services from service catalog: %w", err)
	}
	var edgissaryCandidate *servicecatalog.ServiceSummary
	for _, svc := range serviceSummaries {
		if svc.Cluster == serviceToIntercept.Cluster &&
			(svc.Name == "ambassador" || svc.Name == "edge-stack" || svc.Name == "emissary-ingress") {
			edgissaryCandidate = svc
			break
		}
	}
	if edgissaryCandidate == nil {
		ir.log.Debugf("No edgissary candidate found in cluster %q, returning default ingress info", serviceToIntercept.Cluster)
		return getDefaultIngressInfo(request), nil
	}
	edgissaryInstance, err := ir.serviceCatalog.Get(userIdentity, edgissaryCandidate.ID)
	if err != nil {
		if errors.Is(err, servicecatalog.ServiceNotFoundErr) {
			ir.log.Debugf("Edgissary instance %q was not found, returning default ingress info", edgissaryCandidate.ID)
			return getDefaultIngressInfo(request), nil
		}
		return nil, fmt.Errorf("could not get edgissary instance: %w", err)
	}
	// Try to find which port to route traffic to
	if len(edgissaryInstance.Ports) == 0 {
		ir.log.Debugf("Edgissary service %q has no ports, returning default ingress info", edgissaryInstance.ID)
		return getDefaultIngressInfo(request), nil
	}
	response := &IngressInfoResponse{
		L3Host: fqdn(edgissaryInstance.Name, edgissaryInstance.Namespace),
		L5Host: mappingHostname,
	}
	response.Port, response.UseTLS = resolveIngressPort(edgissaryInstance.Ports)
	return response, nil
}

func getDefaultIngressInfo(request IngressInfoRequest) *IngressInfoResponse {
	response := &IngressInfoResponse{
		L3Host: fqdn(request.ServiceName, request.Namespace),
		Port:   request.ServicePort,
		L5Host: fqdn(request.ServiceName, request.Namespace),
	}
	if request.ServicePortIdentifier == "https" || request.ServicePort == 443 {
		response.UseTLS = true
	} else {
		response.UseTLS = false
	}
	return response
}

func fqdn(name, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace)
}

// findHostname returns a valid hostname that can be used as L5 hosts for the ingress info or an empty string.
// Invalid values are:
//   - HostRegex is true
//   - no Host nor Hostname
//   - Hostname starts with '*'
func findHostname(service *servicecatalog.Service) (string, error) {
	// Unmarshal raw mappings as v3 mappings, which also contain the v2 fields we rely on
	var mappings []mappingV2OrV3
	b, err := json.Marshal(service.Mappings)
	if err != nil {
		return "", fmt.Errorf("could not marshal mappings: %v", err)
	}
	err = json.Unmarshal(b, &mappings)
	if err != nil {
		return "", fmt.Errorf("could not unmarshal mappings: %v", err)
	}
	// Find a valid hostname
	for _, mapping := range mappings {
		if !utils.PointerToBool(mapping.Spec.HostRegex) {
			hostname := mapping.Spec.Hostname
			if hostname == "" {
				hostname = mapping.Spec.Host
			}
			if hostname != "" && !strings.HasPrefix(hostname, "*") {
				return hostname, nil
			}
		}
	}
	return "", fmt.Errorf("no valid hostname was found in the %d mappings", len(mappings))
}

func resolveIngressPort(ports []*models.ServicePort) (int32, bool) {
	var httpsPort int32
	var httpPort int32
	var foundPorts []int32
	for _, p := range ports {
		port := int32(p.Port)
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
