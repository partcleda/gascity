package api

import (
	"strings"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/session"
)

func (s *Server) sessionManager(store beads.Store) *session.Manager {
	cfg := s.state.Config()
	if cfg == nil {
		return session.NewManagerWithCityPath(store, s.state.SessionProvider(), s.state.CityPath())
	}
	return session.NewManagerWithTransportResolverAndCityPath(
		store,
		s.state.SessionProvider(),
		s.state.CityPath(),
		func(template, provider string) string {
			return configuredSessionTransport(cfg, template, provider)
		},
	)
}

func configuredSessionTransport(cfg *config.City, template, provider string) string {
	if cfg == nil {
		return ""
	}
	if agentCfg, ok := resolveSessionTemplateAgent(cfg, template); ok {
		resolved, err := config.ResolveProvider(
			&agentCfg,
			&cfg.Workspace,
			cfg.Providers,
			func(name string) (string, error) { return name, nil },
		)
		if err != nil {
			return strings.TrimSpace(agentCfg.Session)
		}
		return config.ResolveSessionCreateTransport(agentCfg.Session, resolved)
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = strings.TrimSpace(template)
	}
	if provider == "" {
		return ""
	}
	resolved, err := config.ResolveProvider(
		&config.Agent{Provider: provider},
		&cfg.Workspace,
		cfg.Providers,
		func(name string) (string, error) { return name, nil },
	)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(resolved.DefaultSessionTransport())
}
