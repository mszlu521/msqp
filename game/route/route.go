package route

import (
	"core/repo"
	"framework/node"
	"game/handler"
	"game/logic"
)

func Register(r *repo.Manager) node.LogicHandler {
	handlers := make(node.LogicHandler)
	um := logic.NewUnionManager()
	unionHandler := handler.NewUnionHandler(r, um)
	handlers["unionHandler.createRoom"] = unionHandler.CreateRoom
	handlers["unionHandler.joinRoom"] = unionHandler.JoinRoom
	gameHandler := handler.NewGameHandler(r, um)
	handlers["gameHandler.roomMessageNotify"] = gameHandler.RoomMessageNotify
	handlers["gameHandler.gameMessageNotify"] = gameHandler.GameMessageNotify
	return handlers
}
