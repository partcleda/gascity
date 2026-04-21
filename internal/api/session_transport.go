package api

import (
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/runtime"
	sessionacp "github.com/gastownhall/gascity/internal/runtime/acp"
)

type acpRoutingProvider interface {
	RouteACP(name string)
}

func providerSessionTransport(resolved *config.ResolvedProvider, sp runtime.Provider) string {
	if resolved == nil || resolved.DefaultSessionTransport() != "acp" {
		return ""
	}
	if transportSupportsACP(sp) {
		return "acp"
	}
	return ""
}

func transportSupportsACP(sp runtime.Provider) bool {
	if sp == nil {
		return false
	}
	if _, ok := sp.(acpRoutingProvider); ok {
		return true
	}
	_, ok := sp.(*sessionacp.Provider)
	return ok
}
