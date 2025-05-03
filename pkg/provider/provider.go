package provider

import (
	"context"
)

type Provider interface {
	GetName() string
	Prepare() error
	PrepareWithContext(ctx context.Context) error
	Deploy() error
	DeployWithContext(ctx context.Context) error
}

type ProviderFactory interface {
	Create() Provider
}

var registry = make(map[string]ProviderFactory)

func Register(name string, factory ProviderFactory) {
	registry[name] = factory
}

func Get(name string) (Provider, bool) {
	factory, exists := registry[name]
	if !exists {
		return nil, false
	}

	return factory.Create(), true
}

func ListAvailableProviders() []string {
	providers := make([]string, 0, len(registry))
	for name := range registry {
		providers = append(providers, name)
	}
	return providers
}
