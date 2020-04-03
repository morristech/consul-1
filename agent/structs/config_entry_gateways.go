package structs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/acl"
)

// IngressGatewayConfigEntry manages the configuration for an ingress service
// with the given name.
type IngressGatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.IngressGateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	// Listeners declares what ports the ingress gateway should listen on, and
	// what services to associated to those ports.
	Listeners []IngressListener

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

type IngressListener struct {
	// Port declares the port on which the ingress gateway should listen for traffic.
	Port int

	// Protocol declares what type of traffic this listener is expected to
	// receive. Depending on the protocol, a listener might support multiplexing
	// services over a single port, or additional discovery chain features. The
	// current supported values are: (tcp | http).
	Protocol string

	// Services declares the set of services to which the listener forwards
	// traffic.
	//
	// For "tcp" protocol listeners, only a single service is allowed.
	// For "http" listeners, multiple services can be declared.
	Services []IngressService
}

type IngressService struct {
	// Name declares the service to which traffic should be forwarded.
	//
	// This can either be a specific service, or the wildcard specifier,
	// "*". If the wildcard specifier is provided, the listener must be of "http"
	// protocol and means that the listener will forward traffic to all services.
	Name string

	// ServiceSubset declares the specific service subset to which traffic should
	// be sent. This must match an existing service subset declared in a
	// service-resolver config entry.
	ServiceSubset string

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

func (e *IngressGatewayConfigEntry) GetKind() string {
	return IngressGateway
}

func (e *IngressGatewayConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *IngressGatewayConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = IngressGateway
	for i, listener := range e.Listeners {
		if listener.Protocol == "" {
			listener.Protocol = "tcp"
		}

		listener.Protocol = strings.ToLower(listener.Protocol)
		for i := range listener.Services {
			listener.Services[i].EnterpriseMeta.Normalize()
		}

		// Make sure to set the item back into the array, since we are not using
		// pointers to structs
		e.Listeners[i] = listener
	}

	e.EnterpriseMeta.Normalize()

	return nil
}

func (e *IngressGatewayConfigEntry) Validate() error {
	validProtocols := map[string]bool{
		"http": true,
		"tcp":  true,
	}
	declaredPorts := make(map[int]bool)

	for _, listener := range e.Listeners {
		if _, ok := declaredPorts[listener.Port]; ok {
			return fmt.Errorf("port %d declared on two listeners", listener.Port)
		}
		declaredPorts[listener.Port] = true

		if _, ok := validProtocols[listener.Protocol]; !ok {
			return fmt.Errorf("Protocol must be either 'http' or 'tcp', '%s' is an unsupported protocol.", listener.Protocol)
		}

		for _, s := range listener.Services {
			if s.Name == WildcardSpecifier && listener.Protocol != "http" {
				return fmt.Errorf("Wildcard service name is only valid for protocol = 'http' (listener on port %d)", listener.Port)
			}
			if s.Name == "" {
				return fmt.Errorf("Service name cannot be blank (listener on port %d)", listener.Port)
			}
			if s.NamespaceOrDefault() == WildcardSpecifier {
				return fmt.Errorf("Wildcard namespace is not supported for ingress services (listener on port %d)", listener.Port)
			}
		}

		if len(listener.Services) == 0 {
			return fmt.Errorf("No service declared for listener with port %d", listener.Port)
		}

		// Validate that http features aren't being used with tcp or another non-supported protocol.
		if listener.Protocol != "http" && len(listener.Services) > 1 {
			return fmt.Errorf("Multiple services per listener are only supported for protocol = 'http' (listener on port %d)",
				listener.Port)
		}
	}

	return nil
}

func (e *IngressGatewayConfigEntry) CanRead(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ServiceRead(e.Name, &authzContext) == acl.Allow
}

func (e *IngressGatewayConfigEntry) CanWrite(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.OperatorWrite(&authzContext) == acl.Allow
}

func (e *IngressGatewayConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *IngressGatewayConfigEntry) GetEnterpriseMeta() *EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}
