package handler

import (
	"core/dao"
	"core/repo"
	"framework/remote"
)

type UnionHandler struct {
	redisDao *dao.RedisDao
}

// CreateUnion 创建联盟
func (h *UnionHandler) CreateUnion(session *remote.Session, msg []byte) any {

	return nil
}

// JoinUnion 加入联盟
func (h *UnionHandler) JoinUnion(session *remote.Session, msg []byte) any {

	return nil
}

// ExitUnion 退出联盟
func (h *UnionHandler) ExitUnion(session *remote.Session, msg []byte) any {

	return nil
}

// GetUserUnionList 获取用户联盟列表
func (h *UnionHandler) GetUserUnionList(session *remote.Session, msg []byte) any {

	return nil
}

// GetMemberList 获取成员列表
func (h *UnionHandler) GetMemberList(session *remote.Session, msg []byte) any {

	return nil
}

// GetMemberStatisticsInfo 获取成员列表
func (h *UnionHandler) GetMemberStatisticsInfo(session *remote.Session, msg []byte) any {

	return nil
}

// GetMemberScoreList 获取成员列表
func (h *UnionHandler) GetMemberScoreList(session *remote.Session, msg []byte) any {

	return nil
}

// SafeBoxOperation 保险柜操作
func (h *UnionHandler) SafeBoxOperation(session *remote.Session, msg []byte) any {

	return nil
}

// SafeBoxOperationRecord 保险箱操作记录
func (h *UnionHandler) SafeBoxOperationRecord(session *remote.Session, msg []byte) any {

	return nil
}

// ModifyScore 修改积分 count > 0 加分 count < 0 减分
func (h *UnionHandler) ModifyScore(session *remote.Session, msg []byte) any {

	return nil
}

// AddPartner 添加合伙人
func (h *UnionHandler) AddPartner(session *remote.Session, msg []byte) any {

	return nil
}

// GetScoreModifyRecord 查看修改积分日志
func (h *UnionHandler) GetScoreModifyRecord(session *remote.Session, msg []byte) any {

	return nil
}

// InviteJoinUnion 邀请玩家
func (h *UnionHandler) InviteJoinUnion(session *remote.Session, msg []byte) any {

	return nil
}

// OperationInviteJoinUnion 操作俱乐部邀请
func (h *UnionHandler) OperationInviteJoinUnion(session *remote.Session, msg []byte) any {

	return nil
}

// UpdateUnionRebate 更新返利比例
func (h *UnionHandler) UpdateUnionRebate(session *remote.Session, msg []byte) any {

	return nil
}

// UpdateUnionNotice 更新通知
func (h *UnionHandler) UpdateUnionNotice(session *remote.Session, msg []byte) any {

	return nil
}

// GiveScore 赠送积分
func (h *UnionHandler) GiveScore(session *remote.Session, msg []byte) any {

	return nil
}

// GetGiveScoreRecord 赠送积分
func (h *UnionHandler) GetGiveScoreRecord(session *remote.Session, msg []byte) any {

	return nil
}

// GetUnionRebateRecord 获取成员列表
func (h *UnionHandler) GetUnionRebateRecord(session *remote.Session, msg []byte) any {

	return nil
}

// GetGameRecord 获取记录
func (h *UnionHandler) GetGameRecord(session *remote.Session, msg []byte) any {

	return nil
}

// GetVideoRecord 获取游戏录像
func (h *UnionHandler) GetVideoRecord(session *remote.Session, msg []byte) any {

	return nil
}

// UpdateForbidGameStatus 更新禁止游戏状态
func (h *UnionHandler) UpdateForbidGameStatus(session *remote.Session, msg []byte) any {

	return nil
}

// GetRank 排名
func (h *UnionHandler) GetRank(session *remote.Session, msg []byte) any {

	return nil
}

// GetRankSingleDraw 更新单局游戏状态
func (h *UnionHandler) GetRankSingleDraw(session *remote.Session, msg []byte) any {

	return nil
}
func NewUnionHandler(r *repo.Manager) *UnionHandler {
	return &UnionHandler{
		redisDao: dao.NewRedisDao(r),
	}
}
