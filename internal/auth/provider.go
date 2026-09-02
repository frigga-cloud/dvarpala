package auth

import (
	"context"
	"fmt"
	"sort"
)

// UserInfo is what a provider tells us about whoever just logged in.
//
// Only the email matters for authorisation: the provider proves identity, and
// Dvarpala's own database decides whether that identity may have access.
type UserInfo struct {
	Email    string
	Name     string
	Provider string
}

// Provider is one way of proving who someone is.
//
// The flow is the OAuth authorisation-code pattern:
//
//	AuthURL(state)  -> send the browser here
//	   ...user authenticates with the provider...
//	Exchange(code)  -> turn the returned code into an identity
type Provider interface {
	// Name is the URL-safe identifier, e.g. "google".
	Name() string

	// DisplayName is what the sign-in button says, e.g. "Google Workspace".
	DisplayName() string

	// AuthURL is where to send the browser to authenticate.
	AuthURL(state string) string

	// Exchange turns the code handed back by the provider into an identity.
	Exchange(ctx context.Context, code string) (*UserInfo, error)
}

// Registry holds the providers this deployment has enabled.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Add registers a provider.
func (r *Registry) Add(p Provider) {
	r.providers[p.Name()] = p
}

// Get looks up a provider by name.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown or disabled provider: %q", name)
	}
	return p, nil
}

// Enabled lists the registered providers, sorted for stable output.
func (r *Registry) Enabled() []Provider {
	out := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Len reports how many providers are enabled.
func (r *Registry) Len() int { return len(r.providers) }
