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
	handlers["unionHandler.getUserUnionList"] = unionHandler.GetUserUnionList
	handlers["unionHandler.joinUnion"] = unionHandler.JoinUnion
	handlers["unionHandler.exitUnion"] = unionHandler.ExitUnion
	handlers["unionHandler.getMemberList"] = unionHandler.GetMemberList
	handlers["unionHandler.getMemberStatisticsInfo"] = unionHandler.GetMemberStatisticsInfo
	handlers["unionHandler.getMemberScoreList"] = unionHandler.GetMemberScoreList
	handlers["unionHandler.safeBoxOperation"] = unionHandler.SafeBoxOperation
	handlers["unionHandler.safeBoxOperationRecord"] = unionHandler.SafeBoxOperationRecord
	handlers["unionHandler.modifyScore"] = unionHandler.ModifyScore
	handlers["unionHandler.addPartner"] = unionHandler.AddPartner
	handlers["unionHandler.getScoreModifyRecord"] = unionHandler.GetScoreModifyRecord
	handlers["unionHandler.inviteJoinUnion"] = unionHandler.InviteJoinUnion
	handlers["unionHandler.operationInviteJoinUnion"] = unionHandler.OperationInviteJoinUnion
	handlers["unionHandler.updateUnionRebate"] = unionHandler.UpdateUnionRebate
	handlers["unionHandler.updateUnionNotice"] = unionHandler.UpdateUnionNotice
	handlers["unionHandler.giveScore"] = unionHandler.GiveScore
	handlers["unionHandler.getGiveScoreRecord"] = unionHandler.GetGiveScoreRecord
	handlers["unionHandler.getUnionRebateRecord"] = unionHandler.GetUnionRebateRecord
	handlers["unionHandler.getGameRecord"] = unionHandler.GetGameRecord
	handlers["unionHandler.getVideoRecord"] = unionHandler.GetVideoRecord
	handlers["unionHandler.updateForbidGameStatus"] = unionHandler.UpdateForbidGameStatus
	handlers["unionHandler.getRank"] = unionHandler.GetRank
	handlers["unionHandler.getRankSingleDraw"] = unionHandler.GetRankSingleDraw
	gameHandler := handler.NewGameHandler(r)
	handlers["gameHandler.joinRoom"] = gameHandler.JoinRoom
	return handlers
}
