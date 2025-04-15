package provider

import (
	"context"

	"github.com/blazity/enterprise-cli/pkg/logging"
)

type Provider interface {
	GetName() string
	Prepare() error
	PrepareWithContext(ctx context.Context) error
	Deploy() error
	DeployWithContext(ctx context.Context) error
}

type ProviderFactory interface {
	Create(logger logging.Logger) Provider
}

var registry = make(map[string]ProviderFactory)

func Register(name string, factory ProviderFactory) {
	registry[name] = factory
}

func Get(name string, logger logging.Logger) (Provider, bool) {
	factory, exists := registry[name]
	if !exists {
		return nil, false
	}
	
	return factory.Create(logger), true
}

func ListAvailableProviders() []string {
	providers := make([]string, 0, len(registry))
	for name := range registry {
		providers = append(providers, name)
	}
	return providers
}