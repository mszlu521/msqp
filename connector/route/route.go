package route

import (
	"connector/handler"
	"core/repo"
	"framework/net"
)

func Register(r *repo.Manager) net.LogicHandler {
	handlers := make(net.LogicHandler)
	entryHandler := handler.NewEntryHandler(r)
	handlers["entryHandler.entry"] = entryHandler.Entry

	return handlers
}
