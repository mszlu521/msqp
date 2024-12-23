package logic

import (
	"common/biz"
	"common/logs"
	"common/utils"
	"context"
	"core/models/entity"
	"core/models/enums"
	"core/service"
	"encoding/json"
	"framework/msError"
	"framework/remote"
	"game/component/proto"
	"game/component/room"
	"game/models/request"
	"game/models/response"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strconv"
	"sync"
	"time"
)

type Union struct {
	sync.RWMutex
	Id           int64
	m            *UnionManager
	unionData    *entity.Union
	RoomList     map[string]*room.Room
	unionService *service.UnionService
	activeTime   time.Time
	redisService *service.RedisService
	userService  *service.UserService
}

func (u *Union) DestroyRoom(roomId string) {
	delete(u.RoomList, roomId)
}
func (u *Union) CreateRoom(redisService *service.RedisService, userService *service.UserService, session *remote.Session, req request.CreateRoomReq, userData *entity.User) *msError.Error {
	newRoom, err := u.createRoom(req, userData.Uid, session)
	if err != nil {
		logs.Error("CreateRoom err:%v", err)
		return biz.Fail
	}
	newRoom.UserService = userService
	newRoom.RedisService = redisService
	return newRoom.UserEntryRoom(session, userData)
}
func (u *Union) GetOwnerUid() string {
	if u.unionData == nil {
		return ""
	}
	return u.unionData.OwnerUid
}
func (u *Union) DismissRoom(roomId string, session *remote.Session) {
	u.Lock()
	defer u.Unlock()
	r, ok := u.RoomList[roomId]
	if !ok {
		return
	}
	r.DismissRoom(session, enums.UnionOwnerDismiss)
}

func (u *Union) createRoom(req request.CreateRoomReq, uid string, session *remote.Session) (*room.Room, error) {
	u.Lock()
	defer u.Unlock()
	//1. 需要创建一个房间 生成一个房间号
	roomId := u.m.CreateRoomId()
	gameRule := req.GameRule
	if req.GameRuleID != "" {
		gameRule = u.GetGameRule(req.GameRuleID)
	}
	creatorInfo := &proto.RoomCreator{
		Uid:         uid,
		CreatorType: enums.UserCreatorType,
		UnionID:     u.Id,
	}
	if u.Id != 1 {
		creatorInfo.CreatorType = enums.UnionCreatorType
	}
	newRoom, err := room.NewRoom(roomId, creatorInfo, gameRule, u, session)
	if err != nil {
		return nil, err
	}
	u.RoomList[roomId] = newRoom
	return newRoom, nil
}

func (u *Union) GetUnionInfo(uid string) entity.Union {
	u.activeTime = time.Now()
	unionData := *u.unionData
	if uid != unionData.OwnerUid {
		unionData.JoinRequestList = []entity.JoinRequest{}
	}
	return unionData
}

func (u *Union) init() {
	if u.Id != 1 {
		u.unionData = u.unionService.FindUnionById(u.Id)
	}
}

func (u *Union) GetUnionRoomList() []*proto.RoomInfo {
	u.activeTime = time.Now()
	var list []*proto.RoomInfo
	for _, v := range u.RoomList {
		roomInfo := v.GetRoomInfo()
		list = append(list, roomInfo)
	}
	if len(list) <= 0 {
		list = []*proto.RoomInfo{}
	}
	return list
}

func (u *Union) QuickJoin(session *remote.Session, gameRuleID string, userInfo *entity.User) *msError.Error {
	u.activeTime = time.Now()
	var roomRuleItem *entity.RoomRule
	for _, v := range u.unionData.RoomRuleList {
		if v.Id.Hex() == gameRuleID {
			roomRuleItem = v
			break
		}
	}
	if roomRuleItem == nil {
		return biz.RoomNotExist
	}
	//查询是否有房间 有直接加入
	for _, v := range u.RoomList {
		if v.GameRule.Id == gameRuleID &&
			v.CanEnter() && v.HasEmptyChair() {
			return u.JoinRoom(session, v.Id, userInfo)
		}
	}
	//创建房间
	roomId := u.m.CreateRoomId()
	creatorInfo := &proto.RoomCreator{
		Uid:         userInfo.Uid,
		CreatorType: enums.UserCreatorType,
		UnionID:     u.Id,
	}
	if u.Id != 1 {
		creatorInfo.CreatorType = enums.UnionCreatorType
	}
	var gameRule proto.GameRule
	err := json.Unmarshal([]byte(roomRuleItem.GameRule), &gameRule)
	if err != nil {
		logs.Error("QuickJoin json.Unmarshal err:%v", err)
		return biz.Fail
	}
	gameRule.GameType = enums.GameType(roomRuleItem.GameType)
	gameRule.RuleName = roomRuleItem.RuleName
	gameRule.Id = roomRuleItem.Id.Hex()
	roomFrame, err := room.NewRoom(roomId, creatorInfo, gameRule, u, session)
	if err != nil {
		logs.Error("QuickJoin NewRoom err:%v", err)
		return biz.Fail
	}
	u.RoomList[roomId] = roomFrame
	roomFrame.UpdateLotteryInfo(u.getLotteryStatus())
	return roomFrame.UserEntryRoom(session, userInfo)
}

func (u *Union) AddRoomRuleList(gameRule proto.GameRule, ruleName string, gameType int) error {
	marshal, _ := json.Marshal(gameRule)
	roomRule := &entity.RoomRule{
		Id:       primitive.NewObjectID(),
		GameRule: string(marshal),
		GameType: gameType,
		RuleName: ruleName,
	}
	matchData := bson.M{"unionID": u.Id}
	saveData := bson.M{"$push": bson.M{"roomRuleList": roomRule}}
	unionData, err := u.unionService.FindUnionAndUpdate(context.Background(), matchData, saveData)
	if err != nil {
		return err
	}
	if unionData == nil {
		return nil
	}
	u.unionData.RoomRuleList = unionData.RoomRuleList
	return nil
}

func (u *Union) UpdateRoomRuleList(id string, gameRule proto.GameRule, ruleName string, gameType int) error {
	marshal, _ := json.Marshal(gameRule)
	saveData := bson.M{"$set": bson.M{
		"roomRuleList.$.gameRule": string(marshal),
		"roomRuleList.$.ruleName": ruleName,
		"roomRuleList.$.gameType": gameType,
	}}
	objectId, err2 := primitive.ObjectIDFromHex(id)
	if err2 != nil {
		logs.Info("UpdateRoomRuleList ObjectIDFromHex err:%v", err2)
		return err2
	}
	matchData := bson.M{"unionID": u.Id, "roomRuleList._id": objectId}
	unionData, err := u.unionService.FindUnionAndUpdate(context.Background(), matchData, saveData)
	if err != nil {
		return err
	}
	if unionData == nil {
		return nil
	}
	u.unionData.RoomRuleList = unionData.RoomRuleList
	return nil
}

func (u *Union) JoinRoom(session *remote.Session, roomId string, data *entity.User) *msError.Error {
	// 俱乐部房间，则判断该玩家是否在该俱乐部
	if u.Id != 1 {
		var item *entity.UnionInfo
		for _, v := range data.UnionInfo {
			if v.UnionID == u.Id {
				item = v
			}
		}
		if item == nil {
			return biz.NotInUnion
		}
	}
	u.activeTime = time.Now()
	roomFrame := u.RoomList[roomId]
	if roomFrame == nil {
		return biz.RoomNotExist
	}
	return roomFrame.JoinRoom(session, data)
}

func (u *Union) getLotteryStatus() *entity.ResultLotteryInfo {
	if u.unionData == nil {
		return &entity.ResultLotteryInfo{}
	}
	return u.unionData.ResultLotteryInfo
}

func (u *Union) GetHongBao(uid string) (*response.HongBaoResp, *msError.Error) {
	r := &response.HongBaoResp{}
	r.Code = biz.OK
	if u.unionData == nil {
		r.Msg = map[string]any{
			"score": -1,
		}
		return r, nil
	}
	honeBaoInfo := u.unionData.HongBaoInfo
	if honeBaoInfo == nil || !honeBaoInfo.Status {
		r.Msg = map[string]any{
			"score": -1,
		}
		return r, nil
	}
	now := time.Now()
	hour := now.Hour()
	if int64(hour) < honeBaoInfo.StartTime || int64(hour) >= honeBaoInfo.EndTime {
		r.Msg = map[string]any{
			"score": -1,
		}
		return r, nil
	}
	if honeBaoInfo.TotalScore <= 0 || honeBaoInfo.Count <= 0 {
		r.Msg = map[string]any{
			"score": -1,
		}
		return r, nil
	}
	if len(u.unionData.HongBaoScoreList) <= 0 {
		r.Msg = map[string]any{
			"score": 0,
		}
		return r, nil
	}
	if utils.IndexOf(u.unionData.HongBaoUidList, uid) != -1 {
		r.Msg = map[string]any{
			"score": 0,
		}
		return r, nil
	}
	score, _, err := utils.Shift(u.unionData.HongBaoScoreList)
	if err != nil {
		logs.Error("GetHongBao Shift err:%v", err)
		return nil, biz.Fail
	}
	u.unionData.HongBaoUidList = append(u.unionData.HongBaoUidList, uid)
	saveData := bson.M{"$inc": bson.M{
		"unionInfo.$.score": score,
	}}
	matchData := bson.M{"unionInfo.unionID": u.Id, "uid": uid}
	newUserData := u.userService.UpdateUserData(matchData, saveData)
	var newUnionInfo *entity.UnionInfo
	for _, v := range newUserData.UnionInfo {
		if v.UnionID == u.Id {
			newUnionInfo = v
		}
	}
	if newUnionInfo == nil {
		r.Msg = map[string]any{
			"score": -1,
		}
		return r, nil
	}
	scoreChangeRecord := &entity.UserScoreChangeRecord{
		Uid:              uid,
		Nickname:         newUserData.Nickname,
		UnionID:          u.Id,
		ChangeCount:      int64(score),
		LeftCount:        int64(newUnionInfo.Score),
		LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
		ChangeType:       enums.ScoreChangeNone,
		Describe:         "领取红包:" + strconv.Itoa(int(score)),
		CreateTime:       time.Now().UnixMilli(),
	}
	err = u.userService.SaveUserScoreChangeRecord(scoreChangeRecord)
	if err != nil {
		logs.Error("GetHongBao SaveUserScoreChangeRecord err:%v", err)
		return nil, biz.Fail
	}
	_, err = u.unionService.FindUnionAndUpdate(
		context.Background(),
		bson.M{"unionID": u.Id},
		bson.M{
			"$set": bson.M{
				"hongBaoScoreList": u.unionData.HongBaoScoreList,
			},
			"$push": bson.M{
				"hongBaoUidList": uid,
			},
		})
	if err != nil {
		logs.Error("GetHongBao FindUnionAndUpdate err:%v", err)
		return nil, biz.Fail
	}
	r.Msg = map[string]any{
		"score": score,
	}
	r.UpdateUserData = map[string]any{
		"unionInfo": newUserData.UnionInfo,
	}
	return r, nil
}

func (u *Union) UpdateOpeningStatus(open bool) {
	if u.unionData == nil {
		return
	}
	if u.unionData.Opening == open {
		return
	}
	saveData := bson.M{"$set": bson.M{"opening": open}}
	_, err := u.unionService.FindUnionAndUpdate(context.Background(), bson.M{"unionID": u.Id}, saveData)
	if err != nil {
		logs.Error("UpdateOpeningStatus FindUnionAndUpdate err:%v", err)
	}
	u.unionData.Opening = open
}

func (u *Union) RemoveRoomRuleList(roomRuleId string) {
	if u.unionData == nil {
		return
	}
	data, err := u.unionService.FindUnionAndUpdate(
		context.Background(),
		bson.M{"unionID": u.Id},
		bson.M{"$pull": bson.M{"roomRuleList._id": roomRuleId}},
	)
	if err != nil {
		logs.Error("RemoveRoomRuleList FindUnionAndUpdate err:%v", err)
		return
	}
	u.unionData.RoomRuleList = data.RoomRuleList
}
func (u *Union) IsOpening() bool {
	if u.unionData == nil {
		return false
	}
	return u.unionData.Opening
}

// GetLastActiveTime 获取上次活跃时间
func (u *Union) GetLastActiveTime() time.Time {
	return u.activeTime
}

func (u *Union) IsShouldDelete(t int64) bool {
	return len(u.RoomList) == 0 && u.activeTime.UnixMilli()-time.Now().UnixMilli() > t
}

func (u *Union) UpdateUnionNotice(notice string) {
	if u.unionData == nil {
		return
	}
	if u.unionData.Notice == notice {
		return
	}
	saveData := bson.M{"$set": bson.M{"notice": notice}}
	_, err := u.unionService.FindUnionAndUpdate(context.Background(), bson.M{"unionID": u.Id}, saveData)
	if err != nil {
		logs.Error("UpdateUnionNotice FindUnionAndUpdate err:%v", err)
		return
	}
	u.unionData.Notice = notice
}

func (u *Union) GetGameRule(gameRuleID string) proto.GameRule {
	for _, v := range u.unionData.RoomRuleList {
		logs.Info("gameRuleID:%s,RoomRuleList ruleId: %s", gameRuleID, v.Id.String())
		if v.Id.Hex() == gameRuleID {
			rule := v.GameRule
			var gameRule proto.GameRule
			_ = json.Unmarshal([]byte(rule), &gameRule)
			gameRule.Id = gameRuleID
			return gameRule
		}
	}
	return proto.GameRule{}
}

func (u *Union) UpdateUnionName(unionName string) {
	if u.unionData == nil {
		return
	}
	if u.unionData.UnionName == unionName {
		return
	}
	saveData := bson.M{"$set": bson.M{"unionName": unionName}}
	_, err := u.unionService.FindUnionAndUpdate(context.Background(), bson.M{"unionID": u.Id}, saveData)
	if err != nil {
		logs.Error("UpdateUnionName FindUnionAndUpdate err:%v", err)
		return
	}
	u.unionData.UnionName = unionName
}

func (u *Union) UpdatePartnerNoticeSwitch(isOpen bool) {
	if u.unionData == nil {
		return
	}
	if u.unionData.NoticeSwitch == isOpen {
		return
	}
	saveData := bson.M{"$set": bson.M{"noticeSwitch": isOpen}}
	_, err := u.unionService.FindUnionAndUpdate(context.Background(), bson.M{"unionID": u.Id}, saveData)
	if err != nil {
		logs.Error("UpdatePartnerNoticeSwitch FindUnionAndUpdate err:%v", err)
		return
	}
	u.unionData.NoticeSwitch = isOpen
}

func (u *Union) UpdateHongBaoSetting(status bool, startTime int64, endTime int64, count int, totalScore int64) *msError.Error {
	if u.unionData == nil {
		return biz.Fail
	}
	hongbaoInfo := &entity.HongBaoInfo{
		Count:      int32(count),
		EndTime:    endTime,
		StartTime:  startTime,
		Status:     status,
		TotalScore: int32(totalScore),
	}
	updateInfo := bson.M{
		"$set": bson.M{"hongBaoInfo": hongbaoInfo},
	}
	// 关闭红包时，清除所有红包
	if !status {
		u.unionData.HongBaoScoreList = []int32{}
		u.unionData.HongBaoUidList = []string{}
		updateInfo["$set"].(bson.M)["hongBaoScoreList"] = u.unionData.HongBaoScoreList
		updateInfo["$set"].(bson.M)["hongBaoUidList"] = u.unionData.HongBaoUidList
	} else {
		// 红包数为零时重新分派红包
		if len(u.unionData.HongBaoScoreList) == 0 {
			u.unionData.HongBaoScoreList = utils.RandomRedPacket(hongbaoInfo.TotalScore, count)
			u.unionData.HongBaoUidList = []string{}
			updateInfo["$set"].(bson.M)["hongBaoScoreList"] = u.unionData.HongBaoScoreList
			updateInfo["$set"].(bson.M)["hongBaoUidList"] = u.unionData.HongBaoUidList
		} else {
			return biz.CanNotCreateNewHongBao
		}
	}
	_, err := u.unionService.FindUnionAndUpdate(context.Background(), bson.M{"unionID": u.Id}, updateInfo)
	if err != nil {
		logs.Error("UpdateHongBaoSetting FindUnionAndUpdate err:%v", err)
		return biz.Fail
	}
	u.unionData.HongBaoInfo = hongbaoInfo
	return nil
}

func (u *Union) UpdateLotteryStatus(isOpen bool) {
	if u.unionData == nil {
		return
	}
	if u.unionData.ResultLotteryInfo == nil {
		u.unionData.ResultLotteryInfo = &entity.ResultLotteryInfo{}
	}
	if u.unionData.ResultLotteryInfo.Status == isOpen {
		return
	}
	u.unionData.ResultLotteryInfo.Status = isOpen
	saveData := bson.M{"$set": bson.M{"resultLotteryInfo": u.unionData.ResultLotteryInfo}}
	_, err := u.unionService.FindUnionAndUpdate(context.Background(), bson.M{"unionID": u.Id}, saveData)
	if err != nil {
		logs.Error("UpdateLotteryStatus FindUnionAndUpdate err:%v", err)
	}
	for _, v := range u.RoomList {
		v.UpdateLotteryInfo(u.unionData.ResultLotteryInfo)
	}
}
func NewUnion(m *UnionManager, unionID int64, unionService *service.UnionService, redisService *service.RedisService, userService *service.UserService) *Union {
	return &Union{
		RoomList:     make(map[string]*room.Room),
		m:            m,
		Id:           unionID,
		unionService: unionService,
		redisService: redisService,
		userService:  userService,
	}
}
