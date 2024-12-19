package handler

import (
	"common"
	"common/biz"
	"common/logs"
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
	"strconv"
	"time"
)

type UnionHandler struct {
	redisDao    *dao.RedisDao
	userDao     *dao.UserDao
	unionDao    *dao.UnionDao
	recordDao   *dao.RecordDao
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
	return common.S(res)
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
		CreateTime: time.Now().Unix(),
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
		CreateTime:       time.Now().Unix(),
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
	now := time.Now().Unix()
	matchData := bson.M{
		"uid":     session.GetUid(),
		"unionID": req.UnionID,
		"createTime": bson.M{
			"$gte": now - int64(WEEK_MS/time.Second), // 将 WEEK_MS 转换为秒
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
		CreateTime:   time.Now().Unix(),
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
		CreateTime:       time.Now().Unix(),
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
		CreateTime:       time.Now().Unix(),
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

	return common.S(nil)
}

// GetScoreModifyRecord 查看修改积分日志
func (h *UnionHandler) GetScoreModifyRecord(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// InviteJoinUnion 邀请玩家
func (h *UnionHandler) InviteJoinUnion(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// OperationInviteJoinUnion 操作俱乐部邀请
func (h *UnionHandler) OperationInviteJoinUnion(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// UpdateUnionRebate 更新返利比例
func (h *UnionHandler) UpdateUnionRebate(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// UpdateUnionNotice 更新通知
func (h *UnionHandler) UpdateUnionNotice(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// GiveScore 赠送积分
func (h *UnionHandler) GiveScore(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// GetGiveScoreRecord 赠送积分
func (h *UnionHandler) GetGiveScoreRecord(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// GetUnionRebateRecord 获取成员列表
func (h *UnionHandler) GetUnionRebateRecord(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// GetGameRecord 获取记录
func (h *UnionHandler) GetGameRecord(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// GetVideoRecord 获取游戏录像
func (h *UnionHandler) GetVideoRecord(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// UpdateForbidGameStatus 更新禁止游戏状态
func (h *UnionHandler) UpdateForbidGameStatus(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// GetRank 排名
func (h *UnionHandler) GetRank(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

// GetRankSingleDraw 更新单局游戏状态
func (h *UnionHandler) GetRankSingleDraw(session *remote.Session, msg []byte) any {

	return common.S(nil)
}

func NewUnionHandler(r *repo.Manager) *UnionHandler {
	return &UnionHandler{
		redisDao:    dao.NewRedisDao(r),
		userDao:     dao.NewUserDao(r),
		unionDao:    dao.NewUnionDao(r),
		recordDao:   dao.NewRecordDao(r),
		userService: service.NewUserService(r),
	}
}
