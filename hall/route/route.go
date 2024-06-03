package route

import (
	"core/repo"
	"framework/node"
	"hall/handler"
)

func Register(r *repo.Manager) node.LogicHandler {
	handlers := make(node.LogicHandler)
	userHandler := handler.NewUserHandler(r)
	handlers["userHandler.updateUserAddress"] = userHandler.UpdateUserAddress
	handlers["userHandler.bindPhone"] = userHandler.BindPhone
	handlers["userHandler.authRealName"] = userHandler.AuthRealName
	handlers["userHandler.searchByPhone"] = userHandler.SearchByPhone
	handlers["userHandler.searchUserData"] = userHandler.SearchUserData
	unionHandler := handler.NewUnionHandler(r)
	handlers["unionHandler.createUnion"] = unionHandler.CreateUnion
	gameHandler := handler.NewGameHandler(r)
	handlers["gameHandler.joinRoom"] = gameHandler.JoinRoom
	return handlers
}
