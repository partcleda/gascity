package api

import (
	"testing"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/runtime"
)

type createTransportCapableProvider struct {
	*runtime.Fake
}

func (p *createTransportCapableProvider) SupportsTransport(transport string) bool {
	return transport == "acp"
}

func TestProviderSessionTransportUsesExplicitACPConfigOnCustomProvider(t *testing.T) {
	transport, err := providerSessionTransport(&config.ResolvedProvider{
		Name:        "custom-acp",
		SupportsACP: true,
		ACPCommand:  "/bin/echo",
	}, &createTransportCapableProvider{Fake: runtime.NewFake()})
	if err != nil {
		t.Fatalf("providerSessionTransport: %v", err)
	}
	if transport != "acp" {
		t.Fatalf("providerSessionTransport() = %q, want %q", transport, "acp")
	}
}

func TestProviderSessionTransportSupportsACPAloneStaysDefault(t *testing.T) {
	transport, err := providerSessionTransport(&config.ResolvedProvider{
		Name:        "custom-acp",
		SupportsACP: true,
	}, &createTransportCapableProvider{Fake: runtime.NewFake()})
	if err != nil {
		t.Fatalf("providerSessionTransport: %v", err)
	}
	if transport != "" {
		t.Fatalf("providerSessionTransport() = %q, want empty transport", transport)
	}
}

func TestResolveSessionTemplateForCreateUsesProviderACPDefault(t *testing.T) {
	fs := newSessionFakeState(t)
	supportsACP := true
	fs.cfg = &config.City{
		Workspace: config.Workspace{Name: "test-city"},
		Agents: []config.Agent{{
			Name:     "worker",
			Dir:      "myrig",
			Provider: "custom-acp",
		}},
		Providers: map[string]config.ProviderSpec{
			"custom-acp": {
				Command:     "/bin/echo",
				PathCheck:   "true",
				SupportsACP: &supportsACP,
				ACPCommand:  "/bin/echo",
				ACPArgs:     []string{"acp"},
			},
		},
	}

	srv := New(fs)
	_, _, transport, _, err := srv.resolveSessionTemplateForCreate("myrig/worker")
	if err != nil {
		t.Fatalf("resolveSessionTemplateForCreate: %v", err)
	}
	if transport != "acp" {
		t.Fatalf("transport = %q, want %q", transport, "acp")
	}
}

func TestResolveSessionTemplateKeepsLegacyRuntimeTransportDefault(t *testing.T) {
	fs := newSessionFakeState(t)
	supportsACP := true
	fs.cfg = &config.City{
		Workspace: config.Workspace{Name: "test-city"},
		Agents: []config.Agent{{
			Name:     "worker",
			Dir:      "myrig",
			Provider: "custom-acp",
		}},
		Providers: map[string]config.ProviderSpec{
			"custom-acp": {
				Command:     "/bin/echo",
				PathCheck:   "true",
				SupportsACP: &supportsACP,
				ACPCommand:  "/bin/echo",
				ACPArgs:     []string{"acp"},
			},
		},
	}

	srv := New(fs)
	_, _, transport, _, err := srv.resolveSessionTemplate("myrig/worker")
	if err != nil {
		t.Fatalf("resolveSessionTemplate: %v", err)
	}
	if transport != "" {
		t.Fatalf("transport = %q, want empty runtime default", transport)
	}
}

func TestConfiguredSessionTransportUsesProviderACPDefaultForAgentTemplates(t *testing.T) {
	supportsACP := true
	cfg := &config.City{
		Workspace: config.Workspace{Name: "test-city"},
		Agents: []config.Agent{{
			Name:     "worker",
			Dir:      "myrig",
			Provider: "custom-acp",
		}},
		Providers: map[string]config.ProviderSpec{
			"custom-acp": {
				Command:     "/bin/echo",
				PathCheck:   "true",
				SupportsACP: &supportsACP,
				ACPCommand:  "/bin/echo",
				ACPArgs:     []string{"acp"},
			},
		},
	}

	transport := configuredSessionTransport(cfg, "myrig/worker", "")
	if transport != "acp" {
		t.Fatalf("configuredSessionTransport() = %q, want %q", transport, "acp")
	}
}
