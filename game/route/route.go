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
	handlers["unionHandler.getUnionInfo"] = unionHandler.GetUnionInfo
	handlers["unionHandler.getUnionRoomList"] = unionHandler.GetUnionRoomList
	handlers["unionHandler.quickJoin"] = unionHandler.QuickJoin
	handlers["unionHandler.getHongBao"] = unionHandler.GetHongBao
	unionMgrHandler := handler.NewUnionMgrHandler(r, um)
	handlers["unionMgrHandler.addRoomRuleList"] = unionMgrHandler.AddRoomRuleList
	handlers["unionMgrHandler.updateRoomRuleList"] = unionMgrHandler.UpdateRoomRuleList
	handlers["unionMgrHandler.removeRoomRuleList"] = unionMgrHandler.RemoveRoomRuleList
	handlers["unionMgrHandler.updateOpeningStatus"] = unionMgrHandler.UpdateOpeningStatus
	handlers["unionMgrHandler.updateUnionNotice"] = unionMgrHandler.UpdateUnionNotice
	handlers["unionMgrHandler.updateUnionName"] = unionMgrHandler.UpdateUnionName
	handlers["unionMgrHandler.updatePartnerNoticeSwitch"] = unionMgrHandler.UpdatePartnerNoticeSwitch
	handlers["unionMgrHandler.dismissRoom"] = unionMgrHandler.DismissRoom
	handlers["unionMgrHandler.hongBaoSetting"] = unionMgrHandler.HongBaoSetting
	handlers["unionMgrHandler.updateLotteryStatus"] = unionMgrHandler.UpdateLotteryStatus
	gameHandler := handler.NewGameHandler(r, um)
	handlers["gameHandler.roomMessageNotify"] = gameHandler.RoomMessageNotify
	handlers["gameHandler.gameMessageNotify"] = gameHandler.GameMessageNotify
	return handlers
}
