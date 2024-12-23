package handler

import (
	"common"
	"common/biz"
	"common/logs"
	"common/utils"
	"context"
	"core/dao"
	"core/models/entity"
	"core/models/enums"
	"core/repo"
	"core/service"
	"encoding/json"
	"fmt"
	"framework/game"
	"framework/remote"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"hall/models/request"
	"hall/models/response"
	"math"
	"strconv"
	"time"
)

type UnionHandler struct {
	redisDao    *dao.RedisDao
	userDao     *dao.UserDao
	unionDao    *dao.UnionDao
	recordDao   *dao.RecordDao
	commonDao   *dao.CommonDao
	userService *service.UserService
}

// CreateUnion 创建联盟
func (h *UnionHandler) CreateUnion(session *remote.Session, msg []byte) any {
	var req request.CreateUnionReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	//长度限制一下
	if len(req.UnionName) > 60 {
		return common.F(biz.RequestDataError)
	}
	user, err := h.userDao.FindUserByUid(context.TODO(), session.GetUid())
	if err != nil {
		return common.F(biz.InvalidUsers)
	}
	if user == nil {
		return common.F(biz.InvalidUsers)
	}
	//不管用户是否有权限，直接创建
	//是否有创建联盟的资格 开发阶段让每个用户都有资格
	user.IsAgent = true
	if !user.IsAgent {
		return common.F(biz.RequestDataError)
	}
	unionConfig := game.Conf.GameConfig["unionConfig"]
	valueMap := unionConfig["value"].(map[string]any)
	userMaxUnionCount := int(valueMap["userMaxUnionCount"].(float64))
	if len(user.UnionInfo) > userMaxUnionCount {
		return common.F(biz.RequestDataError)
	}
	// 判断是否已经创建过牌友圈，如果已经创建过，则无法重复创建
	union, err := h.unionDao.FindUnionListByUId(context.Background(), user.Uid)
	if err != nil {
		logs.Error("[UnionHandler] FindUnionListByUId find union err:%v", err)
		return common.F(biz.SqlError)
	}
	if union != nil {
		return common.F(biz.AlreadyCreatedUnion)
	}
	unionId, err := h.redisDao.NextUnionId()
	if err != nil {
		logs.Error("[UnionHandler] NextUnionId redis next union id err:%v", err)
		return common.F(biz.SqlError)
	}
	union = &entity.Union{
		UnionID:          unionId,
		OwnerUid:         user.Uid,
		OwnerNickname:    user.Nickname,
		OwnerAvatar:      user.Avatar,
		UnionName:        req.UnionName,
		CurMember:        1,
		OnlineMember:     1,
		CreateTime:       time.Now().UnixMilli(),
		RoomRuleList:     []*entity.RoomRule{},
		JoinRequestList:  []entity.JoinRequest{},
		HongBaoScoreList: []int32{},
		HongBaoUidList:   []string{},
		HongBaoInfo:      &entity.HongBaoInfo{},
	}
	_, err = h.unionDao.Insert(context.Background(), union)
	if err != nil {
		logs.Error("[UnionHandler] CreateUnion insert union err:%v", err)
		return common.F(biz.SqlError)
	}
	unionInfo := &entity.UnionInfo{
		UnionID:    union.UnionID,
		Partner:    true,
		RebateRate: 1,
		JoinTime:   time.Now().UnixMilli(),
	}
	inviteId, err := h.redisDao.NextInviteId()
	if err != nil {
		return common.F(biz.SqlError)
	}
	unionInfo.InviteID = inviteId
	match := bson.M{"uid": user.Uid}
	save := bson.M{"$addToSet": bson.M{"unionInfo": unionInfo}}
	newUserData, err := h.userDao.FindAndUpdate(context.Background(), match, save)
	if err != nil {
		logs.Error("[UnionHandler] CreateUnion update user err:%v", err)
		return common.F(biz.SqlError)
	}
	res := &response.CreateUnionResp{}
	res.Code = biz.OK
	res.UpdateUserData = map[string]any{
		"updateUserData": map[string]any{
			"unionInfo": newUserData.UnionInfo,
		},
	}
	res.Msg = map[string]any{
		"unionID": union.UnionID,
	}
	return res
}

// JoinUnion 加入联盟
func (h *UnionHandler) JoinUnion(session *remote.Session, msg []byte) any {
	var req request.JoinUnionReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.InviteID == "" {
		return common.F(biz.RequestDataError)
	}
	inviteID, _ := strconv.ParseInt(req.InviteID, 10, 64)
	inviteUserData, err := h.userDao.FindUserByInviteID(context.Background(), inviteID)
	if err != nil {
		logs.Error("[UnionHandler] JoinUnion find user by invite id err:%v", err)
		return common.F(biz.InviteIdError)
	}
	if inviteUserData == nil {
		return common.F(biz.InviteIdError)
	}
	var inviteUnionInfo *entity.UnionInfo
	for _, v := range inviteUserData.UnionInfo {
		if v.InviteID == inviteID {
			inviteUnionInfo = v
			break
		}
	}
	unionData, err := h.unionDao.FindUnionByUnionID(context.Background(), inviteUnionInfo.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] JoinUnion find union err:%v", err)
		return common.F(biz.UnionNotExist)
	}
	if unionData == nil {
		return common.F(biz.UnionNotExist)
	}
	if unionData.ForbidInvite && session.GetUid() != unionData.OwnerUid {
		return common.F(biz.ForbidInviteScore)
	}
	userData, err := h.userDao.FindUserByUid(context.Background(), session.GetUid())
	if err != nil {
		return common.F(biz.InvalidUsers)
	}
	for _, v := range userData.UnionInfo {
		if v.UnionID == inviteUnionInfo.UnionID {
			return common.F(biz.AlreadyInUnion)
		}
	}
	// 更新联盟数据
	_, err = h.unionDao.FindAndUpdate(context.Background(), bson.M{
		"unionID": inviteUnionInfo.UnionID,
	}, bson.M{
		"$inc": bson.M{"curMember": 1},
	})
	if err != nil {
		logs.Error("[UnionHandler] JoinUnion update union curMember err:%v", err)
		return common.F(biz.SqlError)
	}
	addUnionInfo := entity.UnionInfo{
		UnionID:    inviteUnionInfo.UnionID,
		SpreaderID: inviteUserData.Uid,
		JoinTime:   time.Now().UnixMilli(),
	}
	inviteId, err := h.redisDao.NextInviteId()
	if err != nil {
		logs.Error("[UnionHandler] NextInviteId err:%v", err)
		return common.F(biz.SqlError)
	}
	addUnionInfo.InviteID = inviteId
	match := bson.M{"uid": session.GetUid()}
	save := bson.M{"$push": bson.M{"unionInfo": addUnionInfo}}
	newUserData, err := h.userDao.FindAndUpdate(context.Background(), match, save)
	if err != nil {
		logs.Error("[UnionHandler] JoinUnion update user err:%v", err)
		return common.F(biz.SqlError)
	}
	res := &response.JoinUnionResp{}
	res.Code = biz.OK
	res.UpdateUserData = map[string]any{
		"updateUserData": map[string]any{
			"unionInfo": newUserData.UnionInfo,
		},
	}
	res.Msg = map[string]any{
		"unionID": inviteUnionInfo.UnionID,
	}
	return res
}

// ExitUnion 退出联盟
func (h *UnionHandler) ExitUnion(session *remote.Session, msg []byte) any {
	var req request.ExitUnionReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	union, err := h.unionDao.FindUnionByUnionID(context.Background(), req.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] ExitUnion find union err:%v", err)
		return common.F(biz.SqlError)
	}
	if union == nil {
		return common.F(biz.UnionNotExist)
	}
	if union.OwnerUid == session.GetUid() {
		// 盟主不能退出联盟，必须将联盟转移给他认之后
		return common.F(biz.RequestDataError)
	}
	userData, err := h.userDao.FindUserByMatchData(context.Background(), bson.M{
		"unionInfo.unionID": req.UnionID,
		"uid":               session.GetUid(),
	})
	if err != nil {
		logs.Error("[UnionHandler] ExitUnion find user err:%v", err)
		return common.F(biz.SqlError)
	}
	if userData == nil {
		return common.F(biz.NotInUnion)
	}
	var userInfoItem *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			userInfoItem = v
			break
		}
	}
	if userInfoItem == nil {
		return common.F(biz.NotInUnion)
	}
	// 删除联盟数据
	newUserData, err := h.userDao.FindAndUpdate(context.Background(), bson.M{
		"uid": session.GetUid(),
	}, bson.M{
		"$pull": bson.M{
			"unionInfo": bson.M{
				"unionID": req.UnionID,
			},
		},
	})
	if err != nil {
		logs.Error("[UnionHandler] ExitUnion update user err:%v", err)
		return common.F(biz.SqlError)
	}
	// 将下级用户转移给上级玩家
	err = h.userDao.UpdateAllData(context.Background(), bson.M{
		"unionInfo": bson.M{
			"$elemMatch": bson.M{
				"unionID":    req.UnionID,
				"spreaderID": session.GetUid(),
			},
		},
	}, bson.M{
		"$set": bson.M{
			"unionInfo.$.spreaderID": userInfoItem.SpreaderID,
		},
	})
	if err != nil {
		logs.Error("[UnionHandler] ExitUnion update user spreaderID err:%v", err)
		return common.F(biz.SqlError)
	}
	// 更新俱乐部人数
	_, err = h.unionDao.FindAndUpdate(context.Background(), bson.M{
		"unionID": req.UnionID,
	}, bson.M{
		"$inc": bson.M{"curMember": -1},
	})
	if err != nil {
		logs.Error("[UnionHandler] ExitUnion update union curMember err:%v", err)
		return common.F(biz.SqlError)
	}
	res := &response.ExitUnionResp{
		Code: biz.OK,
		UpdateUserData: map[string]any{
			"unionInfo": newUserData.UnionInfo,
		},
	}
	return res
}

// GetUserUnionList 获取用户联盟列表
func (h *UnionHandler) GetUserUnionList(session *remote.Session, msg []byte) any {
	uid := session.GetUid()
	if uid == "" {
		return common.F(biz.InvalidUsers)
	}
	user, err := h.userDao.FindUserByUid(context.TODO(), uid)
	if err != nil {
		return common.F(biz.InvalidUsers)
	}
	var unionIDList []int64
	for _, v := range user.UnionInfo {
		unionIDList = append(unionIDList, v.UnionID)
	}
	if len(unionIDList) == 0 {
		res := common.S(&response.UnionListResp{RecordArr: []response.UnionRecord{}})
		return res
	}
	list, err := h.unionDao.FindUnionListByIds(context.Background(), unionIDList)
	if err != nil {
		return common.F(biz.SqlError)
	}
	var unionRecords []response.UnionRecord
	for _, v := range list {
		unionRecords = append(unionRecords, response.UnionRecord{
			UnionID:       v.UnionID,
			UnionName:     v.UnionName,
			OwnerUid:      v.OwnerUid,
			OwnerNickname: v.OwnerNickname,
			OwnerAvatar:   v.OwnerAvatar,
			MemberCount:   v.CurMember,
			OnlineCount:   v.OnlineMember,
		})
	}
	result := &response.UnionListResp{
		RecordArr: unionRecords,
	}
	res := common.S(result)
	return res
}

// GetMemberList 获取成员列表
func (h *UnionHandler) GetMemberList(session *remote.Session, msg []byte) any {
	var req request.MemberListReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	list, total, err := h.userDao.FindUserPage(context.Background(), req.StartIndex, req.Count, bson.M{
		"roomID":          -1,
		"frontendId":      -1,
		"unionInfo.score": -1,
	}, req.MatchData)
	if err != nil {
		logs.Error("[UnionHandler] GetMemberList find user page err:%v", err)
		return common.F(biz.SqlError)
	}
	var memberList []*response.UnionMember
	for _, v := range list {
		score := 0
		safeScore := 0
		var unionInfoItem *entity.UnionInfo
		for _, v1 := range v.UnionInfo {
			if v1.UnionID == req.UnionID {
				unionInfoItem = v1
				score = v1.Score
				safeScore = v1.SafeScore
				break
			}
		}
		if unionInfoItem == nil {
			return common.F(biz.RequestDataError)
		}
		memberList = append(memberList, &response.UnionMember{
			Uid:                       v.Uid,
			Nickname:                  v.Nickname,
			Avatar:                    v.Avatar,
			RoomId:                    v.RoomID,
			FrontendId:                v.FrontendId,
			SpreaderID:                unionInfoItem.SpreaderID,
			Score:                     score,
			SafeScore:                 safeScore,
			ProhibitGame:              unionInfoItem.ProhibitGame,
			YesterdayDraw:             unionInfoItem.YesterdayDraw,
			YesterdayBigWinDraw:       unionInfoItem.YesterdayBigWinDraw,
			YesterdayRebate:           unionInfoItem.YesterdayRebate,
			TodayRebate:               unionInfoItem.TodayRebate,
			MemberYesterdayDraw:       unionInfoItem.MemberYesterdayDraw,
			MemberYesterdayBigWinDraw: unionInfoItem.MemberYesterdayBigWinDraw,
			YesterdayProvideRebate:    unionInfoItem.YesterdayProvideRebate,
			TotalDraw:                 unionInfoItem.TotalDraw,
			RebateRate:                unionInfoItem.RebateRate,
		})
	}
	if memberList == nil {
		memberList = []*response.UnionMember{}
	}
	return common.S(&response.MemberListResp{
		RecordArr:  memberList,
		TotalCount: total,
	})
}

// GetMemberStatisticsInfo 统计
func (h *UnionHandler) GetMemberStatisticsInfo(session *remote.Session, msg []byte) any {
	var req request.MemberStatisticsInfoReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	pipeline := mongo.Pipeline{
		// Unwind the unionInfo array
		{{"$unwind", "$unionInfo"}},

		// Match the given conditions (msg.matchData)
		{{"$match", req.MatchData}},

		// Group by _id and aggregate the data
		{{"$group", bson.M{
			"_id":                         nil, // Grouping by null to get a single document
			"yesterdayTotalDraw":          bson.M{"$sum": "$unionInfo.yesterdayDraw"},
			"yesterdayTotalProvideRebate": bson.M{"$sum": "$unionInfo.yesterdayProvideRebate"},
			"totalCount":                  bson.M{"$sum": 1},
		}}},
	}
	result, err := h.userDao.FindUserAggregateByMatchData(context.Background(), pipeline)
	if err != nil {
		logs.Error("[UnionHandler] GetMemberStatisticsInfo find user aggregate err:%v", err)
		return common.F(biz.SqlError)
	}
	res := &response.MemberStatisticsInfoResp{
		TotalCount:                  0,
		YesterdayTotalDraw:          0,
		YesterdayTotalProvideRebate: 0,
	}
	if len(result) > 0 {
		res.YesterdayTotalProvideRebate = result[0].YesterdayTotalProvideRebate
		res.YesterdayTotalDraw = result[0].YesterdayTotalDraw
		res.TotalCount = result[0].TotalCount
	}
	return common.S(res)
}

// GetMemberScoreList 获取成员列表
func (h *UnionHandler) GetMemberScoreList(session *remote.Session, msg []byte) any {
	var req request.MemberScoreReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	list, total, err := h.userDao.FindUserPage(context.Background(), req.StartIndex, req.Count, bson.M{
		"unionInfo.score": -1,
	}, req.MatchData)
	if err != nil {
		logs.Error("[UnionHandler] GetMemberScoreList find user page err:%v", err)
		return common.F(biz.SqlError)
	}
	var memberList []*response.MemberScoreRecord
	for _, v := range list {
		var unionInfoItem *entity.UnionInfo
		for _, v1 := range v.UnionInfo {
			if v1.UnionID == req.UnionID {
				unionInfoItem = v1
				break
			}
		}
		memberList = append(memberList, &response.MemberScoreRecord{
			Uid:       v.Uid,
			Nickname:  v.Nickname,
			Avatar:    v.Avatar,
			Score:     int32(unionInfoItem.Score),
			SafeScore: int32(unionInfoItem.SafeScore),
		})
	}
	pipeline := mongo.Pipeline{
		// Unwind the unionInfo array
		{{"$unwind", "$unionInfo"}},

		// Match the given conditions (msg.matchData)
		{{"$match", req.MatchData}},

		// Group by _id and aggregate the data
		{{"$group", bson.M{
			"_id":       nil, // Grouping by null to get a single document
			"score":     bson.M{"$sum": "$unionInfo.score"},
			"safeScore": bson.M{"$sum": "$unionInfo.safeScore"},
		}}},
	}
	result, err := h.userDao.FindUserAggregateByMatchData(context.Background(), pipeline)
	if err != nil {
		logs.Error("[UnionHandler] GetMemberScoreList find user aggregate err:%v", err)
		return common.F(biz.SqlError)
	}
	var totalScore int64
	if len(result) > 0 {
		totalScore = result[0].Score + result[0].SafeScore
	}
	return common.S(&response.MemberScoreListResp{
		RecordArr:  memberList,
		TotalCount: total,
		TotalScore: totalScore,
	})
}

// SafeBoxOperation 保险柜操作
func (h *UnionHandler) SafeBoxOperation(session *remote.Session, msg []byte) any {
	var req request.SafeBoxOperation
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	userData, err := h.userDao.GetUnlockUserDataAndLock(context.Background(), session.GetUid())
	if err != nil {
		return common.F(biz.SqlError)
	}
	if userData == nil {
		return common.F(biz.UserDataLocked)
	}
	if userData.RoomID != "" {
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] SafeBoxOperation unlock user data err:%v", err)
		}
		return common.F(biz.UserInRoomDataLocked)
	}
	var unionInfo *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			unionInfo = v
			break
		}
	}
	if unionInfo == nil {
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] SafeBoxOperation unlock user data err:%v", err)
		}
		return common.F(biz.UserInRoomDataLocked)
	}
	// 校验数据,大于0,则是存，小于0则是取
	if (req.Count > 0 && unionInfo.Score < req.Count) || (req.Count < 0 && unionInfo.SafeScore < -req.Count) {
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] SafeBoxOperation unlock user data err:%v", err)
		}
		return common.F(biz.UserInRoomDataLocked)
	}
	// 更新数据
	saveData := bson.M{
		"unionInfo.$.safeScore": req.Count,
		"unionInfo.$.score":     -req.Count,
		"syncLock":              0,
	}
	newUserData, err := h.userDao.FindAndUpdate(context.Background(), bson.M{"uid": userData.Uid, "unionInfo.unionID": unionInfo.UnionID}, saveData)
	if err != nil {
		logs.Error("[UnionHandler] SafeBoxOperation find and update user data err:%v", err)
		return common.F(biz.SqlError)
	}
	saveRecord := &entity.SafeBoxRecord{
		Uid:        session.GetUid(),
		UnionID:    req.UnionID,
		CreateTime: time.Now().UnixMilli(),
		Count:      int32(req.Count),
	}
	err = h.recordDao.CreateSafeBoxOperationRecord(context.Background(), saveRecord)
	if err != nil {
		logs.Error("[UnionHandler] SafeBoxOperation create safe box operation record err:%v", err)
		return common.F(biz.SqlError)
	}
	var newUnionInfo *entity.UnionInfo
	for _, v := range newUserData.UnionInfo {
		if v.UnionID == unionInfo.UnionID {
			newUnionInfo = v
			break
		}
	}
	describe := "存入" + strconv.Itoa(req.Count)
	if req.Count <= 0 {
		describe = "取出" + strconv.Itoa(-req.Count)
	}
	scoreChangeRecord := &entity.UserScoreChangeRecord{
		Uid:              session.GetUid(),
		Nickname:         newUserData.Nickname,
		UnionID:          req.UnionID,
		LeftCount:        int64(newUnionInfo.Score),
		LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
		ChangeCount:      int64(-req.Count),
		ChangeType:       enums.SafeBox,
		CreateTime:       time.Now().UnixMilli(),
		Describe:         describe,
	}
	err = h.recordDao.CreateUserScoreChangeRecord(context.Background(), scoreChangeRecord)
	if err != nil {
		logs.Error("[UnionHandler] SafeBoxOperation create user score change record err:%v", err)
		return common.F(biz.SqlError)
	}
	res := &response.SafeBoxOperationRecord{
		Code: biz.OK,
		UpdateUserData: map[string]any{
			"unionInfo": newUserData.UnionInfo,
		},
	}
	return res
}

const WEEK_MS = 7 * 24 * time.Hour

// SafeBoxOperationRecord 保险箱操作记录
func (h *UnionHandler) SafeBoxOperationRecord(session *remote.Session, msg []byte) any {
	var req request.SafeBoxOperation
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	now := time.Now().UnixMilli()

	matchData := bson.M{
		"uid":     session.GetUid(),
		"unionID": req.UnionID,
		"createTime": bson.M{
			"$gte": now - int64(WEEK_MS/time.Millisecond), // 将 WEEK_MS 转换为毫秒
		},
	}
	sortData := bson.M{
		"createTime": -1,
	}
	recordPage, total, err := h.recordDao.FindSafeBoxOperationRecordPage(context.Background(), req.StartIndex, req.Count, sortData, matchData)
	if err != nil {
		logs.Error("[UnionHandler] SafeBoxOperationRecord find safe box operation record page err:%v", err)
		return common.F(biz.SqlError)
	}
	if len(recordPage) <= 0 {
		recordPage = []*entity.SafeBoxRecord{}
	}
	return common.S(map[string]any{
		"recordArr":  recordPage,
		"totalCount": total,
	})
}

// ModifyScore 修改积分 count > 0 加分 count < 0 减分
func (h *UnionHandler) ModifyScore(session *remote.Session, msg []byte) any {
	var req request.ModifyScoreReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 || req.MemberUid == "" || req.Count <= 0 {
		return common.F(biz.RequestDataError)
	}
	unionData, err := h.unionDao.FindUnionByUnionID(context.Background(), req.UnionID)
	if err != nil {
		return common.F(biz.SqlError)
	}
	if unionData == nil {
		return common.F(biz.RequestDataError)
	}
	if unionData.ForbidGive && unionData.OwnerUid != session.GetUid() {
		return common.F(biz.ForbidGiveScore)
	}
	userData, err := h.userDao.GetUnlockUserDataAndLock(context.Background(), session.GetUid())
	if err != nil {
		return common.F(biz.SqlError)
	}
	if userData == nil {
		return common.F(biz.RequestDataError)
	}
	if userData.RoomID != "" {
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore unlock user data err:%v", err)
		}
		return common.F(biz.UserInRoomDataLocked)
	}
	// 判断是否在联盟中
	var userUnionInfoItem *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			userUnionInfoItem = v
			break
		}
	}
	// 非盟主时需要判断积分是否足够,盟主时不需要判断分数
	if userUnionInfoItem == nil || (userUnionInfoItem.Score < req.Count && session.GetUid() != unionData.OwnerUid) {
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore unlock user data err:%v", err)
		}
		return common.F(biz.RequestDataError)
	}
	// 查询会员数据
	memberData, err := h.userDao.FindUserByUid(context.Background(), req.MemberUid)
	if err != nil {
		logs.Error("[UnionHandler] ModifyScore find user by uid err:%v", err)
		return common.F(biz.SqlError)
	}
	if memberData == nil {
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore unlock user data err:%v", err)
		}
		return common.F(biz.RequestDataError)
	}
	if memberData.RoomID != "" && req.Count < 0 {
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore unlock user data err:%v", err)
		}
		err = h.userDao.UnLockUserData(context.Background(), req.MemberUid)
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore unlock user data err:%v", err)
		}
		return common.F(biz.UserInRoomDataLocked)
	}
	// 判断是否在联盟中
	var memberUnionInfoItem *entity.UnionInfo
	for _, v := range memberData.UnionInfo {
		if v.UnionID == req.UnionID {
			memberUnionInfoItem = v
			break
		}
	}
	if memberUnionInfoItem == nil ||
		memberUnionInfoItem.SpreaderID != session.GetUid() ||
		memberUnionInfoItem.Score < -req.Count {
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore unlock user data err:%v", err)
		}
		err = h.userDao.UnLockUserData(context.Background(), req.MemberUid)
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore unlock user data err:%v", err)
		}
		return common.F(biz.RequestDataError)
	}
	// 修改分数
	var newUserData *entity.User
	// 非盟主时，需改对应修改分数数量
	if session.GetUid() != unionData.OwnerUid {
		matchData := bson.M{
			"uid":               session.GetUid(),
			"unionInfo.unionID": req.UnionID,
		}
		saveData := bson.M{
			"$inc": bson.M{"unionInfo.$.score": -req.Count},
			"$set": bson.M{"syncLock": 0},
		}
		newUserData, err = h.userDao.FindAndUpdate(context.Background(), matchData, saveData)
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore find and update err:%v", err)
			return common.F(biz.SqlError)
		}
	} else {
		// 盟主时，不需要修改
		err := h.userDao.UnLockUserData(context.Background(), session.GetUid())
		if err != nil {
			logs.Error("[UnionHandler] ModifyScore unlock user data err:%v", err)
		}
		newUserData = userData
	}
	matchData := bson.M{
		"uid":               req.MemberUid,
		"unionInfo.unionID": req.UnionID,
	}
	saveData := bson.M{
		"$inc": bson.M{"unionInfo.$.score": req.Count},
		"$set": bson.M{"syncLock": 0},
	}
	newMemberData, err := h.userDao.FindAndUpdate(context.Background(), matchData, saveData)
	if err != nil {
		logs.Error("[UnionHandler] ModifyScore find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	scoreModifyRecord := &entity.ScoreModifyRecord{
		Uid:          session.GetUid(),
		UnionID:      req.UnionID,
		Nickname:     userData.Nickname,
		Avatar:       userData.Avatar,
		GainUid:      req.MemberUid,
		GainNickname: memberData.Nickname,
		Count:        int32(req.Count),
		CreateTime:   time.Now().UnixMilli(),
	}
	err = h.recordDao.SaveScoreModifyRecord(context.Background(), scoreModifyRecord)
	if err != nil {
		logs.Error("[UnionHandler] ModifyScore save score modify record err:%v", err)
		return common.F(biz.SqlError)
	}
	if newMemberData.FrontendId != "" {
		h.userService.UpdateUserDataNotify(newMemberData.Uid, newMemberData.FrontendId, map[string]any{
			"unionInfo": newMemberData.UnionInfo,
		}, session)
	}
	var newUnionInfo *entity.UnionInfo
	for _, v := range newUserData.UnionInfo {
		if v.UnionID == req.UnionID {
			newUnionInfo = v
			break
		}
	}
	var scoreChangeRecordArr []*entity.UserScoreChangeRecord
	userDescribe := fmt.Sprintf("给下级%s加分%d", newMemberData.Uid, req.Count)
	if req.Count <= 0 {
		userDescribe = fmt.Sprintf("给下级%s减分%d", newMemberData.Uid, -req.Count)
	}
	scoreChangeRecordArr = append(scoreChangeRecordArr, &entity.UserScoreChangeRecord{
		Uid:              newUserData.Uid,
		Nickname:         newUserData.Nickname,
		UnionID:          req.UnionID,
		ChangeCount:      int64(-req.Count),
		LeftCount:        int64(newUnionInfo.Score),
		LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
		ChangeType:       enums.ModifyLow,
		Describe:         userDescribe,
		CreateTime:       time.Now().UnixMilli(),
	})
	var newMemberUnionInfo *entity.UnionInfo
	for _, v := range newMemberData.UnionInfo {
		if v.UnionID == req.UnionID {
			newMemberUnionInfo = v
			break
		}
	}
	userMemberDescribe := fmt.Sprintf("上级%s加分%d", newUserData.Uid, req.Count)
	if req.Count <= 0 {
		userMemberDescribe = fmt.Sprintf("上级%s减分%d", newMemberData.Uid, -req.Count)
	}
	scoreChangeRecordArr = append(scoreChangeRecordArr, &entity.UserScoreChangeRecord{
		Uid:              newMemberData.Uid,
		Nickname:         newMemberData.Nickname,
		UnionID:          req.UnionID,
		ChangeCount:      int64(-req.Count),
		LeftCount:        int64(newMemberUnionInfo.Score),
		LeftSafeBoxCount: int64(newMemberUnionInfo.SafeScore),
		ChangeType:       enums.ModifyUp,
		Describe:         userMemberDescribe,
		CreateTime:       time.Now().UnixMilli(),
	})
	_ = h.recordDao.CreateUserScoreChangeRecordList(context.Background(), scoreChangeRecordArr)
	res := map[string]any{
		"code": biz.OK,
		"updateUserData": map[string]any{
			"unionInfo": newUserData.UnionInfo,
		},
	}
	return res
}

// AddPartner 添加合伙人
func (h *UnionHandler) AddPartner(session *remote.Session, msg []byte) any {
	var req request.AddPartnerReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.MemberUid == "" {
		return common.F(biz.RequestDataError)
	}
	unionData, err := h.unionDao.FindUnionByUnionID(context.Background(), req.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] AddPartner find union err:%v", err)
		return common.F(biz.SqlError)
	}
	if unionData == nil {
		return common.F(biz.RequestDataError)
	}
	//查询用户
	userData, err := h.userDao.FindUserByUid(context.Background(), session.GetUid())
	if err != nil {
		logs.Error("[UnionHandler] AddPartner find user err:%v", err)
		return common.F(biz.SqlError)
	}
	if userData == nil {
		return common.F(biz.InvalidUsers)
	}
	var userUnionInfoItem *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			userUnionInfoItem = v
			break
		}
	}
	if userUnionInfoItem == nil {
		return common.F(biz.RequestDataError)
	}
	memberUserData, err := h.userDao.FindUserByUid(context.Background(), req.MemberUid)
	if err != nil {
		logs.Error("[UnionHandler] AddPartner find user err:%v", err)
		return common.F(biz.SqlError)
	}
	if memberUserData == nil {
		return common.F(biz.NotInUnion)
	}
	var memberUnionInfoItem *entity.UnionInfo
	for _, v := range memberUserData.UnionInfo {
		if v.UnionID == req.UnionID {
			memberUnionInfoItem = v
			break
		}
	}
	// 只有上级可以添加未合伙人
	if memberUnionInfoItem == nil || memberUnionInfoItem.SpreaderID != session.GetUid() {
		return common.F(biz.NotInUnion)
	}
	// 已经添加过则直接返回
	if memberUnionInfoItem.Partner {
		return common.S(nil)
	}
	// 更新用户数据
	saveData := bson.M{
		"$set": bson.M{
			"unionInfo.$.partner": true,
		},
	}
	newMemberData, err := h.userDao.FindAndUpdate(context.Background(), bson.M{"uid": req.MemberUid, "unionInfo.unionID": req.UnionID}, saveData)
	if err != nil {
		logs.Error("[UnionHandler] AddPartner find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	if newMemberData.FrontendId != "" {
		h.userService.UpdateUserDataNotify(newMemberData.Uid, newMemberData.FrontendId, map[string]any{
			"unionInfo": newMemberData.UnionInfo,
		}, session)
	}
	return common.S(nil)
}

// GetScoreModifyRecord 查看修改积分日志
func (h *UnionHandler) GetScoreModifyRecord(session *remote.Session, msg []byte) any {
	var req request.GetScoreModifyRecordReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	recordPage, total, err := h.recordDao.FindScoreModifyRecordPage(
		context.Background(),
		req.StartIndex,
		req.Count,
		bson.M{"createTime": -1}, req.MatchData)
	if err != nil {
		logs.Error("[UnionHandler] GetScoreModifyRecord err:%v", err)
		return common.F(biz.SqlError)
	}
	var totalScoreCount int64
	//"$group": bson.M{
	//	"_id":        nil,
	//	"totalCount": bson.M{"$sum": "$count"},
	//},
	execData := mongo.Pipeline{
		{
			{
				"$match", req.MatchData,
			},
		},
		{
			{
				"$group", bson.M{
					"_id":        nil,
					"totalCount": bson.M{"$sum": "$count"},
				},
			},
		},
	}
	var statisticsInfo []*entity.StatisticsResult
	err = h.commonDao.GetStatisticsInfo(context.Background(), "scoreModifyRecord", execData, &statisticsInfo)
	if err == nil && len(statisticsInfo) > 0 {
		totalScoreCount = statisticsInfo[0].TotalCount
	}
	var yesterdayTotalCount int64
	newMatchData := req.MatchData
	newMatchData["createTime"] = bson.M{
		"$gte": time.Now().UnixMilli() - 86400,
		"$lt":  time.Now().UnixMilli(),
	}
	execData = mongo.Pipeline{
		{
			{
				"$match", newMatchData,
			},
		},
		{
			{
				"$group", bson.M{
					"_id":        nil,
					"totalCount": bson.M{"$sum": "$count"},
				},
			},
		},
	}
	var statisticsInfo1 []*entity.StatisticsResult
	err = h.commonDao.GetStatisticsInfo(context.Background(), "scoreModifyRecord", execData, &statisticsInfo)
	if err == nil && len(statisticsInfo1) > 0 {
		yesterdayTotalCount = statisticsInfo1[0].TotalCount
	}
	var todayTotalCount int64
	newMatchData = req.MatchData
	newMatchData["createTime"] = bson.M{
		"$gte": time.Now().UnixMilli(),
	}
	execData = mongo.Pipeline{
		{
			{
				"$match", newMatchData,
			},
		},
		{
			{
				"$group", bson.M{
					"_id":        nil,
					"totalCount": bson.M{"$sum": "$count"},
				},
			},
		},
	}
	var statisticsInfo2 []*entity.StatisticsResult
	err = h.commonDao.GetStatisticsInfo(context.Background(), "scoreModifyRecord", execData, &statisticsInfo2)
	if err == nil && len(statisticsInfo2) > 0 {
		todayTotalCount = statisticsInfo2[0].TotalCount
	}
	if recordPage == nil {
		recordPage = []*entity.ScoreModifyRecord{}
	}
	res := map[string]any{
		"recordArr":           recordPage,
		"totalCount":          total,
		"todayTotalCount":     math.Abs(float64(todayTotalCount)),
		"yesterdayTotalCount": math.Abs(float64(yesterdayTotalCount)),
		"totalScoreCount":     math.Abs(float64(totalScoreCount)),
	}
	return common.S(res)
}

// InviteJoinUnion 邀请玩家
func (h *UnionHandler) InviteJoinUnion(session *remote.Session, msg []byte) any {
	var req request.InviteJoinUnionReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	unionData, err := h.unionDao.FindUnionByUnionID(context.Background(), req.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] InviteJoinUnion find union err:%v", err)
		return common.F(biz.SqlError)
	}
	if unionData == nil {
		return common.F(biz.RequestDataError)
	}
	// 检查邀请权限
	if unionData.ForbidInvite && session.GetUid() != unionData.OwnerUid {
		return common.F(biz.ForbidInviteScore)
	}
	// 查询用户数据
	userData, err := h.userDao.FindUserByUid(context.Background(), session.GetUid())
	if err != nil {
		logs.Error("[UnionHandler] InviteJoinUnion find user err:%v", err)
		return common.F(biz.SqlError)
	}
	if userData == nil {
		return common.F(biz.InvalidUsers)
	}
	// 判断是否在联盟中
	var userUnionInfoItem *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			userUnionInfoItem = v
			break
		}
	}
	if userUnionInfoItem == nil {
		return common.F(biz.RequestDataError)
	}
	// 查询会员数据
	memberUserData, err := h.userDao.FindUserByUid(context.Background(), req.Uid)
	if err != nil {
		logs.Error("[UnionHandler] InviteJoinUnion find member user err:%v", err)
		return common.F(biz.SqlError)
	}
	if memberUserData == nil {
		return common.F(biz.RequestDataError)
	}
	// 判断是否在联盟中
	var memberUnionInfoItem *entity.UnionInfo
	for _, v := range memberUserData.UnionInfo {
		if v.UnionID == req.UnionID {
			memberUnionInfoItem = v
			break
		}
	}
	if memberUnionInfoItem != nil {
		return common.F(biz.AlreadyInUnion)
	}
	// 更新联盟数据
	saveData := bson.M{
		"$inc": bson.M{
			"curMember": 1,
		},
	}
	_, err = h.unionDao.FindAndUpdate(context.Background(), bson.M{
		"unionID": req.UnionID,
	}, saveData)
	if err != nil {
		logs.Error("[UnionHandler] InviteJoinUnion find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	pushData := entity.UnionInfo{
		UnionID:    req.UnionID,
		SpreaderID: session.GetUid(),
		Partner:    req.Partner,
		JoinTime:   time.Now().UnixMilli(),
	}
	pushData.InviteID, err = h.redisDao.NextInviteId()
	if err != nil {
		logs.Error("[UnionHandler] InviteJoinUnion redis next invite id err:%v", err)
		return common.F(biz.SqlError)
	}
	// 加入到联盟中
	newMemberData, err := h.userDao.FindAndUpdate(context.Background(), bson.M{
		"uid": req.Uid,
	}, bson.M{
		"$push": bson.M{
			"unionInfo": pushData,
		},
	})
	if err != nil {
		logs.Error("[UnionHandler] InviteJoinUnion find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	if newMemberData.FrontendId != "" {
		h.userService.UpdateUserDataNotify(newMemberData.Uid, newMemberData.FrontendId, map[string]any{
			"unionInfo": newMemberData.UnionInfo,
		}, session)
	}
	return common.S(nil)
}

// OperationInviteJoinUnion 操作俱乐部邀请
func (h *UnionHandler) OperationInviteJoinUnion(session *remote.Session, msg []byte) any {
	var req request.OperationInviteJoinUnionReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	userData, err := h.userDao.FindUserByUid(context.Background(), session.GetUid())
	if err != nil {
		logs.Error("[UnionHandler] OperationInviteJoinUnion find user err:%v", err)
		return common.F(biz.SqlError)
	}
	var inviteMsgItem *entity.InviteMsg
	if userData.InviteMsg == nil {
		return common.F(biz.RequestDataError)
	}
	for _, v := range userData.InviteMsg {
		if v.UnionId == req.UnionID && v.Uid == req.Uid {
			inviteMsgItem = v
			break
		}
	}
	if inviteMsgItem == nil {
		return common.F(biz.RequestDataError)
	}
	// 拒绝则直接删除邀请信息
	if !req.Agree {
		newUserData, err := h.userDao.FindAndUpdate(
			context.Background(),
			bson.M{
				"uid": session.GetUid(),
			},
			bson.M{
				"$pull": bson.M{
					"inviteMsg": bson.M{
						"unionId": req.UnionID,
						"uid":     req.Uid,
					},
				},
			},
		)
		if err != nil {
			logs.Error("[UnionHandler] OperationInviteJoinUnion find and update err:%v", err)
			return common.F(biz.SqlError)
		}
		res := map[string]any{
			"updateUserData": map[string]any{
				"inviteMsg": newUserData.InviteMsg,
			},
			"code": biz.OK,
		}
		return res
	}
	// 判断是否已经达到玩家最大联盟数
	if len(userData.UnionInfo) >= 20 {
		return common.F(biz.RequestDataError)
	}
	// 查询联盟数据
	unionData, err := h.unionDao.FindUnionByUnionID(context.Background(), req.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] OperationInviteJoinUnion find union err:%v", err)
		return common.F(biz.SqlError)
	}
	// 联盟不存在，则删除该联盟的信息
	if unionData == nil {
		newUserData, err := h.userDao.FindAndUpdate(
			context.Background(),
			bson.M{
				"uid": session.GetUid(),
			},
			bson.M{
				"$pull": bson.M{
					"inviteMsg": bson.M{
						"unionId": req.UnionID,
					},
				},
			},
		)
		if err != nil {
			logs.Error("[UnionHandler] OperationInviteJoinUnion find and update err:%v", err)
			return common.F(biz.SqlError)
		}
		res := map[string]any{
			"updateUserData": map[string]any{
				"inviteMsg": newUserData.InviteMsg,
			},
			"code": biz.RequestDataError,
		}
		return res
	}
	// 如果已经在联盟中，则删除联盟信息
	var unionInfoItem *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			unionInfoItem = v
			break
		}
	}
	if unionInfoItem != nil {
		newUserData, err := h.userDao.FindAndUpdate(
			context.Background(),
			bson.M{
				"uid": session.GetUid(),
			},
			bson.M{
				"$pull": bson.M{
					"inviteMsg": bson.M{
						"unionId": req.UnionID,
					},
				},
			},
		)
		if err != nil {
			logs.Error("[UnionHandler] OperationInviteJoinUnion find and update err:%v", err)
			return common.F(biz.SqlError)
		}
		res := map[string]any{
			"updateUserData": map[string]any{
				"inviteMsg": newUserData.InviteMsg,
			},
			"code": biz.OK,
		}
		return res
	}
	// 更新联盟数据
	saveData := bson.M{
		"$inc": bson.M{
			"curMember": 1,
		},
	}
	_, err = h.unionDao.FindAndUpdate(context.Background(), bson.M{
		"unionID": req.UnionID,
	}, saveData)
	if err != nil {
		logs.Error("[UnionHandler] OperationInviteJoinUnion find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	// 加入到联盟中
	pushData := entity.UnionInfo{
		UnionID:    req.UnionID,
		SpreaderID: req.Uid,
		Partner:    req.Partner,
		JoinTime:   time.Now().UnixMilli(),
	}
	pushData.InviteID, err = h.redisDao.NextInviteId()
	if err != nil {
		logs.Error("[UnionHandler] InviteJoinUnion redis next invite id err:%v", err)
		return common.F(biz.SqlError)
	}
	// 加入到联盟中
	newUserData, err := h.userDao.FindAndUpdate(context.Background(), bson.M{
		"uid": req.Uid,
	}, bson.M{
		"$push": bson.M{
			"unionInfo": pushData,
		},
		"$pull": bson.M{
			"inviteMsg": bson.M{
				"unionId": req.UnionID,
			},
		},
	})
	if err != nil {
		logs.Error("[UnionHandler] InviteJoinUnion find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	res := map[string]any{
		"updateUserData": map[string]any{
			"inviteMsg": newUserData.InviteMsg,
			"unionInfo": newUserData.UnionInfo,
		},
		"code": biz.OK,
	}
	return res
}

// UpdateUnionRebate 更新返利比例
func (h *UnionHandler) UpdateUnionRebate(session *remote.Session, msg []byte) any {
	var req request.UpdateUnionRebateReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.MemberUid != "" || req.RebateRate > 1 || req.RebateRate < 0 {
		return common.F(biz.RequestDataError)
	}
	// 查询联盟数据
	unionData, err := h.unionDao.FindUnionByUnionID(context.Background(), req.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] UpdateUnionRebate find union err:%v", err)
		return common.F(biz.SqlError)
	}
	if unionData == nil {
		return common.F(biz.RequestDataError)
	}
	// 查询用户数据
	userData, err := h.userDao.FindUserByUid(context.Background(), session.GetUid())
	if err != nil {
		logs.Error("[UnionHandler] UpdateUnionRebate find user err:%v", err)
		return common.F(biz.SqlError)
	}
	if userData == nil {
		return common.F(biz.RequestDataError)
	}
	// 判断是否在联盟中
	var userUnionInfoItem *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			userUnionInfoItem = v
			break
		}
	}
	if userUnionInfoItem == nil || userUnionInfoItem.RebateRate < req.RebateRate {
		return common.F(biz.RequestDataError)
	}
	// 查询会员数据
	memberUserData, err := h.userDao.FindUserByUid(context.Background(), req.MemberUid)
	if err != nil {
		logs.Error("[UnionHandler] UpdateUnionRebate find user err:%v", err)
		return common.F(biz.SqlError)
	}
	if memberUserData == nil {
		return common.F(biz.UserInRoomDataLocked)
	}
	// 判断是否在联盟中，并且是否是该会员的上级用户
	var memberUnionInfoItem *entity.UnionInfo
	for _, v := range memberUserData.UnionInfo {
		if v.UnionID == req.UnionID {
			memberUnionInfoItem = v
			break
		}
	}
	if memberUnionInfoItem == nil ||
		memberUnionInfoItem.SpreaderID != session.GetUid() ||
		memberUnionInfoItem.RebateRate >= req.RebateRate {
		return common.F(biz.RequestDataError)
	}
	// 更新比例
	saveData := bson.M{
		"$set": bson.M{
			"unionInfo.$.rebateRate": req.RebateRate,
		},
	}
	newMemberData, err := h.userDao.FindAndUpdate(context.Background(), bson.M{
		"uid":               req.MemberUid,
		"unionInfo.unionID": req.UnionID,
	}, saveData)
	if err != nil {
		logs.Error("[UnionHandler] UpdateUnionRebate find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	if newMemberData.FrontendId != "" {
		h.userService.UpdateUserDataNotify(newMemberData.Uid, newMemberData.FrontendId, map[string]any{
			"unionInfo": newMemberData.UnionInfo,
		}, session)
	}
	return common.S(nil)
}

// UpdateUnionNotice 更新通知
func (h *UnionHandler) UpdateUnionNotice(session *remote.Session, msg []byte) any {
	var req request.UpdateUnionNoticeReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if len(req.Notice) > 120 {
		return common.F(biz.RequestDataError)
	}
	newUserData, err := h.userDao.FindAndUpdate(context.Background(), bson.M{
		"uid":               session.GetUid(),
		"unionInfo.unionID": req.UnionID,
	}, bson.M{
		"unionInfo.$.notice": req.Notice,
	})
	if err != nil {
		logs.Error("[UnionHandler] UpdateUnionNotice find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	res := map[string]any{
		"updateUserData": map[string]any{
			"unionInfo": newUserData.UnionInfo,
		},
		"code": biz.OK,
	}
	return res
}

// GiveScore 赠送积分
func (h *UnionHandler) GiveScore(session *remote.Session, msg []byte) any {
	var req request.GiveScoreReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.Count <= 0 || req.GiveUid == session.GetUid() {
		return common.F(biz.RequestDataError)
	}
	// 查询联盟数据
	unionData, err := h.unionDao.FindUnionByUnionID(context.Background(), req.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] GiveScore find union err:%v", err)
		return common.F(biz.SqlError)
	}
	if unionData == nil {
		return common.F(biz.RequestDataError)
	}
	// 查看赠送权限
	if unionData.ForbidGive && session.GetUid() != unionData.OwnerUid {
		return common.F(biz.ForbidGiveScore)
	}
	// 查询用户数据
	userData, err := h.userDao.FindUserByUid(context.Background(), session.GetUid())
	if err != nil {
		logs.Error("[UnionHandler] GiveScore find user err:%v", err)
		return common.F(biz.SqlError)
	}
	if userData == nil || userData.RoomID != "" {
		return common.F(biz.RequestDataError)
	}
	// 判断是否在联盟中
	var userUnionInfoItem *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			userUnionInfoItem = v
			break
		}
	}
	// 非盟主时需要判断积分是否足够,盟主时不需要判断分数
	if userUnionInfoItem == nil || (userUnionInfoItem.Score < req.Count && session.GetUid() != unionData.OwnerUid) {
		h.userDao.UnLockUserData(context.Background(), session.GetUid())
		return common.F(biz.RequestDataError)
	}
	// 查询会员数据
	memberUserData, err := h.userDao.FindUserByUid(context.Background(), req.GiveUid)
	if err != nil {
		logs.Error("[UnionHandler] GiveScore find user err:%v", err)
		return common.F(biz.SqlError)
	}
	if memberUserData == nil || memberUserData.RoomID != "" {
		return common.F(biz.InvalidUsers)
	}
	// 判断是否在联盟中
	var memberUnionInfoItem *entity.UnionInfo
	for _, v := range memberUserData.UnionInfo {
		if v.UnionID == req.UnionID {
			memberUnionInfoItem = v
			break
		}
	}
	if memberUnionInfoItem == nil {
		h.userDao.UnLockUserData(context.Background(), session.GetUid())
		return common.F(biz.NotInUnion)
	}
	// 修改分数
	var newUserData *entity.User
	// 非盟主时，需改对应修改分数数量
	if session.GetUid() != unionData.OwnerUid {
		saveData := bson.M{
			"$inc": bson.M{
				"unionInfo.$.score": -req.Count,
			},
		}
		newUserData, err = h.userDao.FindAndUpdate(context.Background(), bson.M{
			"uid":               session.GetUid(),
			"unionInfo.unionID": req.UnionID,
		}, saveData)
		if err != nil {
			logs.Error("[UnionHandler] GiveScore find and updateerr:%v", err)
		}
	} else {
		// 盟主时，不需要修改
		h.userDao.UnLockUserData(context.Background(), session.GetUid())
		newUserData = userData
	}
	newMemberData, err := h.userDao.FindAndUpdate(context.Background(), bson.M{
		"uid":               req.GiveUid,
		"unionInfo.unionID": req.UnionID,
	}, bson.M{
		"$inc": bson.M{
			"unionInfo.$.score": req.Count,
		},
	})
	if err != nil {
		logs.Error("[UnionHandler] GiveScore find and update err:%v", err)
		return common.F(biz.SqlError)
	}
	createData := &entity.ScoreGiveRecord{
		Uid:          session.GetUid(),
		Nickname:     newUserData.Nickname,
		GainUid:      req.GiveUid,
		GainNickname: memberUserData.Nickname,
		UnionID:      req.UnionID,
		Count:        int32(req.Count),
		CreateTime:   time.Now().UnixMilli(),
	}
	err = h.recordDao.SaveScoreGiveRecord(context.Background(), createData)
	if err != nil {
		logs.Error("[UnionHandler] GiveScore save record err:%v", err)
	}
	if newMemberData.FrontendId != "" {
		h.userService.UpdateUserDataNotify(newMemberData.Uid, newMemberData.FrontendId, map[string]any{
			"unionInfo": newMemberData.UnionInfo,
		}, session)
	}
	// 存储分数变化记录
	var newUnionInfo *entity.UnionInfo
	for _, v := range newUserData.UnionInfo {
		if v.UnionID == req.UnionID {
			newUnionInfo = v
			break
		}
	}
	h.recordDao.CreateUserScoreChangeRecord(context.Background(), &entity.UserScoreChangeRecord{
		Uid:              newUserData.Uid,
		Nickname:         newUserData.Nickname,
		UnionID:          req.UnionID,
		ChangeCount:      int64(-req.Count),
		LeftCount:        int64(newUnionInfo.Score),
		LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
		ChangeType:       enums.Give,
		Describe:         fmt.Sprintf("赠送给%s:%d", memberUserData.Uid, req.Count),
		CreateTime:       time.Now().UnixMilli(),
	})
	var newMemberUnionInfo *entity.UnionInfo
	for _, v := range newMemberData.UnionInfo {
		if v.UnionID == req.UnionID {
			newMemberUnionInfo = v
			break
		}
	}
	h.recordDao.CreateUserScoreChangeRecord(context.Background(), &entity.UserScoreChangeRecord{
		Uid:              newMemberData.Uid,
		Nickname:         newMemberData.Nickname,
		UnionID:          req.UnionID,
		ChangeCount:      int64(req.Count),
		LeftCount:        int64(newMemberUnionInfo.Score),
		LeftSafeBoxCount: int64(newMemberUnionInfo.SafeScore),
		ChangeType:       enums.Give,
		Describe:         fmt.Sprintf("%s赠送:%d", newUserData.Uid, req.Count),
		CreateTime:       time.Now().UnixMilli(),
	})
	res := map[string]any{
		"code": biz.OK,
		"updateUserData": map[string]any{
			"unionInfo": newUserData.UnionInfo,
		},
	}
	return res
}

// GetGiveScoreRecord 赠送积分
func (h *UnionHandler) GetGiveScoreRecord(session *remote.Session, msg []byte) any {
	var req request.GetGiveScoreRecordReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	var list []entity.ScoreGiveRecord
	total, err := h.commonDao.FindDataAndCount(
		context.Background(),
		"scoreGiveRecord",
		req.StartIndex, req.Count, bson.M{"createTime": -1}, req.MatchData, &list)
	if err != nil {
		logs.Error("[UnionHandler] GetGiveScoreRecord err:%v", err)
		return common.F(biz.SqlError)
	}
	var totalGiveCount int64
	if total > 0 {
		execData := mongo.Pipeline{
			bson.D{
				{"$match", bson.D{
					{"uid", session.GetUid()},
					{"unionID", req.UnionID},
				}},
			},
			bson.D{
				{"$group", bson.D{
					{"_id", nil},
					{"totalGiveCount", bson.D{
						{"$sum", "$count"},
					}},
				}},
			},
		}
		var statisticsInfo []*entity.StatisticsResult
		err = h.commonDao.GetStatisticsInfo(context.Background(), "scoreGiveRecord", execData, &statisticsInfo)
		if err != nil {
			logs.Error("[UnionHandler] GetGiveScoreRecord err:%v", err)
			return common.F(biz.SqlError)
		}
		if len(statisticsInfo) > 0 {
			totalGiveCount = statisticsInfo[0].TotalGiveCount
		}
	}
	var totalGainCount int64
	if total > 0 {
		execData := mongo.Pipeline{
			bson.D{
				{"$match", bson.D{
					{"gainUid", session.GetUid()},
					{"unionID", req.UnionID},
				}},
			},
			bson.D{
				{"$group", bson.D{
					{"_id", nil},
					{"totalGiveCount", bson.D{
						{"$sum", "$count"},
					}},
				}},
			},
		}
		var statisticsInfo []*entity.StatisticsResult
		err = h.commonDao.GetStatisticsInfo(context.Background(), "scoreGiveRecord", execData, &statisticsInfo)
		if err != nil {
			logs.Error("[UnionHandler] GetGiveScoreRecord err:%v", err)
			return common.F(biz.SqlError)
		}
		if len(statisticsInfo) > 0 {
			totalGainCount = statisticsInfo[0].TotalGiveCount
		}
	}
	if list == nil {
		list = []entity.ScoreGiveRecord{}
	}
	res := map[string]any{
		"recordArr":      list,
		"totalCount":     total,
		"totalGiveCount": totalGiveCount,
		"totalGainCount": totalGainCount,
	}
	return common.S(res)
}

// GetUnionRebateRecord 获取成员列表
func (h *UnionHandler) GetUnionRebateRecord(session *remote.Session, msg []byte) any {
	var req request.GetUnionRebateRecordReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	var list []*entity.UserRebateRecord
	total, err := h.commonDao.FindDataAndCount(
		context.Background(),
		"userRebateRecord",
		req.StartIndex, req.Count, bson.M{"createTime": -1}, req.MatchData, &list)
	if err != nil {
		logs.Error("[UnionHandler] GetUnionRebateRecord err:%v", err)
		return common.F(biz.SqlError)
	}
	if list == nil {
		list = []*entity.UserRebateRecord{}
	}
	res := map[string]any{
		"recordArr":  list,
		"totalCount": total,
	}
	return common.S(res)
}

// GetGameRecord 获取记录
func (h *UnionHandler) GetGameRecord(session *remote.Session, msg []byte) any {
	var req request.GetGameRecordReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	list, total, err := h.recordDao.FindUserGameRecordPage(context.TODO(), req.StartIndex, req.Count, bson.M{"createTime": -1}, req.MatchData)
	if err != nil {
		logs.Error("[UnionHandler] GetGameRecord err:%v", err)
		return common.F(biz.SqlError)
	}
	if list == nil {
		list = []*entity.UserGameRecord{}
	}
	res := map[string]any{
		"recordArr":  list,
		"totalCount": total,
	}
	return common.S(res)
}

// GetVideoRecord 获取游戏录像
func (h *UnionHandler) GetVideoRecord(session *remote.Session, msg []byte) any {
	var req request.GetVideoRecordReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	matchData := bson.M{
		"videoRecordID": req.VideoRecordID,
	}
	var data *entity.GameVideoRecord
	err := h.commonDao.FindOneData(context.Background(), "gameVideoRecord", matchData, data)
	if err != nil {
		logs.Error("[UnionHandler] GetVideoRecord err:%v", err)
		return common.F(biz.SqlError)
	}

	res := map[string]any{
		"gameVideoRecordData": data,
	}
	return common.S(res)
}

// UpdateForbidGameStatus 更新禁止游戏状态
func (h *UnionHandler) UpdateForbidGameStatus(session *remote.Session, msg []byte) any {
	var req request.UpdateForbidGameStatusReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	var unionData *entity.Union
	err := h.commonDao.FindOneData(context.Background(), "union", bson.M{
		"unionID": req.UnionID,
	}, unionData)
	if err != nil {
		logs.Error("[UnionHandler] UpdateForbidGameStatus err:%v", err)
		return common.F(biz.SqlError)
	}
	if unionData == nil || unionData.OwnerUid != session.GetUid() {
		return common.F(biz.RequestDataError)
	}
	// 查询用户数据
	userData, err := h.userDao.FindUserByUid(context.TODO(), session.GetUid())
	if err != nil {
		logs.Error("[UnionHandler] UpdateForbidGameStatus err:%v", err)
		return common.F(biz.SqlError)
	}
	if userData == nil {
		return common.F(biz.InvalidUsers)
	}
	var unionItem *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == req.UnionID {
			unionItem = v
		}
	}
	if unionItem == nil {
		return common.F(biz.RequestDataError)
	}
	if unionItem.ProhibitGame == req.Forbid {
		return common.S(nil)
	}
	newUserData, err := h.userDao.FindAndUpdate(context.TODO(), bson.M{"uid": session.GetUid(), "unionInfo.unionID": req.UnionID}, bson.M{"unionInfo.$.prohibitGame": req.Forbid})
	if err != nil {
		logs.Error("[UnionHandler] UpdateForbidGameStatus err:%v", err)
		return common.F(biz.SqlError)
	}
	if newUserData.FrontendId != "" {
		h.userService.UpdateUserDataNotify(newUserData.Uid, newUserData.FrontendId, map[string]any{
			"unionInfo": newUserData.UnionInfo,
		}, session)
	}
	return common.S(nil)
}

// GetRank 排名
func (h *UnionHandler) GetRank(session *remote.Session, msg []byte) any {
	var req request.GetRankReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	// 查询联盟数据
	unionData, err := h.unionDao.FindUnionByUnionID(context.TODO(), req.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] GetRank err:%v", err)
		return common.F(biz.SqlError)
	}
	if unionData == nil || (unionData.OwnerUid != session.GetUid() && !unionData.ShowRank) {
		return common.F(biz.RequestDataError)
	}
	aggregateData := mongo.Pipeline{
		bson.D{
			{"$match", req.MatchData},
		},
		bson.D{
			{"$unwind", "$unionInfo"},
		},
		bson.D{
			{"$match", req.MatchData},
		},
		bson.D{
			{"$sort", req.SortData},
		},
		bson.D{
			{"$skip", req.StartIndex},
		},
		bson.D{
			{"$limit", req.Count},
		},
	}
	var list []*entity.UserAggregate
	err = h.commonDao.GetStatisticsInfo(context.Background(), "user", aggregateData, &list)
	if err != nil {
		logs.Error("[UnionHandler] GetRank err:%v", err)
		return common.F(biz.SqlError)
	}
	var recordArr []map[string]any
	for _, v := range list {
		if v.UnionInfo == nil {
			continue
		}
		recordArr = append(recordArr, map[string]any{
			"uid":       v.Uid,
			"nickname":  v.Nickname,
			"avatar":    v.Avatar,
			"todayDraw": v.UnionInfo.TodayDraw,
			"weekDraw":  v.UnionInfo.WeekDraw,
			"totalDraw": v.UnionInfo.TotalDraw,
		})
	}
	if recordArr == nil {
		recordArr = []map[string]any{}
	}
	return common.S(map[string]any{
		"recordArr": recordArr,
	})
}

// GetRankSingleDraw
func (h *UnionHandler) GetRankSingleDraw(session *remote.Session, msg []byte) any {
	var req request.GetRankSingleDrawReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	// 查询联盟数据
	unionData, err := h.unionDao.FindUnionByUnionID(context.TODO(), req.UnionID)
	if err != nil {
		logs.Error("[UnionHandler] GetRankSingleDraw err:%v", err)
		return common.F(biz.SqlError)
	}
	if unionData == nil || unionData.OwnerUid != session.GetUid() || !unionData.ShowSingleRank {
		return common.F(biz.RequestDataError)
	}
	req.MatchData["createTime"] = bson.M{"$gte": utils.GetTimeTodayStart()}
	groupData := bson.M{
		"_id": "$userList.uid",
		"nickname": bson.M{
			"$first": "$userList.nickname",
		},
		"avatar": bson.M{
			"$first": "$userList.avatar",
		},
	}
	if req.SortData["score"] == 1 {
		groupData["score"] = bson.M{
			"$min": "$userList.score",
		}
	} else {
		groupData["score"] = bson.M{
			"$max": "$userList.score",
		}
	}
	aggregateData := mongo.Pipeline{
		bson.D{
			{"$match", req.MatchData},
		},
		bson.D{
			{"$unwind", "$userList"},
		},
		bson.D{
			{"$match", req.MatchData},
		},
		bson.D{
			{"$group", groupData},
		},
		bson.D{
			{"$sort", req.SortData},
		},
		bson.D{
			{"$skip", req.StartIndex},
		},
		bson.D{
			{"$limit", req.Count},
		},
	}
	var list []*entity.UserGameRecordAggregate
	err = h.commonDao.GetStatisticsInfo(context.Background(), "userGameRecord", aggregateData, &list)
	if err != nil {
		logs.Error("[UnionHandler] GetRankSingleDraw err:%v", err)
		return common.F(biz.SqlError)
	}
	var recordArr []*entity.GameUser
	for _, v := range list {
		v.UserList.Uid = v.UserList.Id.Hex()
		recordArr = append(recordArr, v.UserList)
	}
	if recordArr == nil {
		recordArr = []*entity.GameUser{}
	}
	return common.S(map[string]any{
		"recordArr": recordArr,
	})
}

func NewUnionHandler(r *repo.Manager) *UnionHandler {
	return &UnionHandler{
		redisDao:    dao.NewRedisDao(r),
		userDao:     dao.NewUserDao(r),
		unionDao:    dao.NewUnionDao(r),
		recordDao:   dao.NewRecordDao(r),
		commonDao:   dao.NewCommonDao(r),
		userService: service.NewUserService(r),
	}
}
