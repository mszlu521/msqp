package room

import (
	"common/biz"
	"common/logs"
	"common/tasks"
	"common/utils"
	"context"
	"core/models/entity"
	"core/models/enums"
	"core/service"
	"encoding/json"
	"errors"
	"fmt"
	"framework/msError"
	"framework/remote"
	"framework/stream"
	"game/component/base"
	"game/component/proto"
	"game/models/request"
	"go.mongodb.org/mongo-driver/bson"
	"log"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type TypeRoom int

const (
	TypeRoomNone TypeRoom = iota
	Normal                //匹配
	Continue              //持续
	Hundred               //百人
)

type Room struct {
	sync.RWMutex
	Id                     string
	unionID                int64
	GameRule               proto.GameRule
	users                  map[string]*proto.RoomUser
	RoomCreator            *proto.RoomCreator
	GameFrame              GameFrame
	kickSchedules          map[string]*time.Timer
	startSchedulerID       *tasks.Task
	answerExitSchedule     *tasks.Task
	stopAnswerSchedules    chan struct{}
	union                  base.UnionBase
	roomDismissed          bool //房间是否被解散
	gameStarted            bool //房间是否开始
	askDismiss             []any
	chairCount             int
	currentUserCount       int
	roomType               TypeRoom
	createTime             time.Time
	lastNativeTime         time.Time
	hasFinishedOneBureau   bool
	hasStartedOneBureau    bool
	alreadyCostUserUidArr  []string
	userJoinGameBureau     map[string]int //记录玩家从第几局加入游戏
	maxBureau              int            //最大局数
	curBureau              int            //当前局数
	clearUserArr           map[string]*entity.GameUser
	UserService            *service.UserService
	RedisService           *service.RedisService
	dismissTick            int
	stopStartSchedulerID   chan struct{}
	resultLotteryInfo      *entity.ResultLotteryInfo
	userGetHongBaoCountArr []int
	roomGeneration         atomic.Uint64
	gameActionMu           sync.Mutex
}

func (r *Room) GetGameStarted() bool {
	r.RLock()
	defer r.RUnlock()
	return r.gameStarted
}

func (r *Room) GetMaxBureau() int {
	r.RLock()
	defer r.RUnlock()
	return r.maxBureau
}

func (r *Room) SetCurBureau(curBureau int) {
	r.Lock()
	defer r.Unlock()
	r.curBureau = curBureau
}

func (r *Room) GetCurBureau() int {
	r.RLock()
	defer r.RUnlock()
	return r.curBureau
}

// IsDismissing 正在解散中
func (r *Room) IsDismissing() bool {
	r.RLock()
	defer r.RUnlock()
	return r.askDismiss != nil && len(r.askDismiss) > 0
}

func (r *Room) GetCreator() *proto.RoomCreator {
	return r.RoomCreator
}

// ConcludeGame 游戏结束相关
func (r *Room) ConcludeGame(data []*proto.EndData, session *remote.Session) {
	if !r.gameStarted {
		return
	}
	r.gameStarted = false
	r.hasFinishedOneBureau = true
	for _, v := range r.users {
		v.UserStatus &= ^enums.Playing
		v.UserStatus &= ^enums.Ready
	}
	//记录游戏结果
	r.recordGameResult(data, session)
	//收取固定抽分
	r.calculateRebateWhenStart(session)
	//记录已经付房费的玩家，防止重复
	for _, v := range r.users {
		if v.ChairID >= r.chairCount {
			continue
		}
		if utils.Contains(r.alreadyCostUserUidArr, v.UserInfo.Uid) {
			continue
		}
		r.alreadyCostUserUidArr = append(r.alreadyCostUserUidArr, v.UserInfo.Uid)
	}
	// 收取每小局分数
	r.recordOneDrawResult(data, session)
	// 判断房间是否应该解散
	if r.maxBureau > 0 && r.curBureau >= r.maxBureau {
		if r.GameRule.GameType != enums.DGN {
			r.DismissRoom(session, enums.BureauFinished)
		}
	} else {
		// 移除不满足条件的玩家
		r.clearNonSatisfiedConditionsUser(session)
		// 通知更新所有玩家信息
		r.notifyUpdateAllUserInfo(session)
	}
}
func (r *Room) resetRoom(session *remote.Session) error {
	r.roomGeneration.Add(1)
	r.createTime = time.Now()
	r.lastNativeTime = time.Now()
	r.roomDismissed = false
	r.gameStarted = false
	r.hasFinishedOneBureau = false
	r.hasStartedOneBureau = false
	r.maxBureau = utils.Default(r.GameRule.Bureau, 8)
	r.curBureau = 0
	for _, v := range r.users {
		v.WinScore = 0
	}
	r.clearUserArr = make(map[string]*entity.GameUser)
	var err error
	r.GameFrame, err = NewGameFrame(r.GameRule, r, session)
	if err != nil {
		return err
	}
	for _, v := range r.users {
		go r.addKickScheduleEvent(session, v.Uid)
	}
	return nil
}
func (r *Room) UserReady(uid string, session *remote.Session) {
	r.userReady(uid, session)
}

func (r *Room) EndGame(session *remote.Session) {
	r.gameStarted = false
	for k := range r.users {
		r.users[k].UserStatus = enums.UserStatusNone
	}
}

func (r *Room) UserEntryRoom(
	session *remote.Session,
	data *entity.User,
) *msError.Error {
	r.Lock()
	defer r.Unlock()
	return r.userEntryRoomLocked(session, data)
}

func (r *Room) userEntryRoomLocked(
	session *remote.Session,
	data *entity.User,
) *msError.Error {
	if r.roomDismissed {
		return biz.NotInRoom
	}
	user, ok := r.users[data.Uid]
	//检查是否允许新用户进入
	if !ok && !r.canEnterLocked() {
		return biz.RoomPlayerCountFull
	}
	curUid := session.GetUid()
	r.cancelKickScheduleLocked(curUid)
	//最多6人参加 0-5有6个号
	chairID := r.getEmptyChairIDLocked(data.Uid, false)
	if chairID < 0 {
		return biz.RoomPlayerCountFull
	}
	// 将用户信息转化为对应俱乐部的信息
	userInfo := proto.BuildGameRoomUserInfoWithUnion(data, r.RoomCreator.UnionID, session.GetMsg().ConnectorId)
	if user == nil {
		//检查是否满足进入房间的条件
		code := r.checkEntryRoom(userInfo)
		if code != nil {
			return code
		}
		user = &proto.RoomUser{
			Uid:        userInfo.Uid,
			UserInfo:   userInfo,
			ChairID:    chairID,
			UserStatus: enums.UserStatusNone,
		}
		r.users[data.Uid] = user
		r.currentUserCount++
		//2. 将房间号 推送给客户端 更新数据库 当前房间号存储起来
		err := r.UpdateUserInfoRoomPush(session, data.Uid)
		if err != nil {
			r.rollbackNewUserEntryLocked(data.Uid)
			return biz.SqlError
		}
	} else {
		user.UserInfo = userInfo
		if user.UserStatus&enums.Offline > 0 {
			user.UserStatus &= ^enums.Offline
		}
		//如果有离线倒计时，这里应该取消
	}
	session.Put("roomId", r.Id, stream.Single)
	//存储roomId和服务器的关系
	err := r.RedisService.Store(r.Id, session.GetDst())
	if err != nil {
		if !ok {
			r.rollbackNewUserEntryLocked(data.Uid)
			if rollbackErr := r.UserService.UpdateUserRoomId(context.Background(), data.Uid, ""); rollbackErr != nil {
				logs.Error("rollback user room id failed uid:%s err:%v", data.Uid, rollbackErr)
			}
			session.Put("roomId", "", stream.Single)
			r.sendDataOne(proto.UpdateUserInfoPush(map[string]any{"roomID": ""}), data.Uid, session.GetMsg())
		}
		return biz.SqlError
	}
	//向其他玩家推送进入房间的消息
	roomUserInfo := &proto.RoomUser{
		Uid:        userInfo.Uid,
		UserInfo:   user.UserInfo,
		ChairID:    user.ChairID,
		UserStatus: user.UserStatus,
	}
	r.sendDataExceptUid(proto.OtherUserEntryRoomPushData(roomUserInfo), user.UserInfo.Uid, session.GetMsg())
	// 推送玩家自己进入房间的消息
	r.SelfEntryRoomPush(session, data.Uid)
	r.GameFrame.OnEventUserEntry(user, session)
	go r.addKickScheduleEvent(session, userInfo.Uid)
	return nil
}

func (r *Room) rollbackNewUserEntryLocked(uid string) {
	if _, exists := r.users[uid]; !exists {
		return
	}
	delete(r.users, uid)
	if r.currentUserCount > 0 {
		r.currentUserCount--
	}
}

func (r *Room) UpdateUserInfoRoomPush(session *remote.Session, uid string) error {
	//更新数据库用户信息
	err := r.UserService.UpdateUserRoomId(context.Background(), uid, r.Id)
	if err != nil {
		return err
	}
	//{roomID: '336842', pushRouter: 'UpdateUserInfoPush'}
	pushMsg := map[string]any{
		"roomID":     r.Id,
		"pushRouter": "UpdateUserInfoPush",
	}
	//node节点 nats client，push 通过nats将消息发送给connector服务，connector将消息主动发给客户端
	//ServerMessagePush
	r.sendDataOne(pushMsg, uid, session.GetMsg())
	return nil
}

func (r *Room) SelfEntryRoomPush(session *remote.Session, uid string) {
	//{gameType: 1, pushRouter: 'SelfEntryRoomPush'}
	pushMsg := map[string]any{
		"gameType":   r.GameRule.GameType,
		"pushRouter": "SelfEntryRoomPush",
	}
	r.SendData(session.GetMsg(), []string{uid}, pushMsg)
}

func (r *Room) ReceiveRoomMessage(session *remote.Session, req request.RoomMessageReq) {
	if req.Type == proto.UserReadyNotify {
		r.userReady(session.GetUid(), session)
	}
	if req.Type == proto.UserLeaveRoomNotify {
		r.userLeaveRoomRequest(session)
	}
	if req.Type == proto.GetRoomSceneInfoNotify {
		r.getRoomSceneInfoPush(session)
	}
	if req.Type == proto.AskForDismissNotify {
		r.askForDismiss(session, session.GetUid(), req.Data.IsExit)
	}
	if req.Type == proto.UserChangeSeatNotify {
		r.userChangeSeat(session, req.Data.FromChairID, req.Data.ToChairID)
	}
	if req.Type == proto.UserChatNotify {
		r.userChat(session, req.Data)
	}
}

func (r *Room) getRoomSceneInfoPush(session *remote.Session) {
	//
	userInfoArr := make([]*proto.RoomUser, 0)
	for _, v := range r.GetUsers() {
		userInfoArr = append(userInfoArr, v)
	}
	data := map[string]any{
		"type":       proto.GetRoomSceneInfoPush,
		"pushRouter": "RoomMessagePush",
		"data": map[string]any{
			"roomID":          r.Id,
			"roomCreatorInfo": r.RoomCreator,
			"gameRule":        r.GameRule,
			"roomUserInfoArr": userInfoArr,
			"gameData":        r.GameFrame.GetEnterGameData(session),
		},
	}
	r.sendDataOne(data, session.GetUid(), session.GetMsg())
	if r.askDismiss != nil && len(r.askDismiss) > 0 {
		r.askForDismiss(session, session.GetUid(), nil)
	}
}

func (r *Room) addKickScheduleEvent(session *remote.Session, uid string) {
	r.Lock()
	defer r.Unlock()
	roomUser, hasUser := r.users[uid]
	if !hasUser || roomUser == nil || roomUser.UserInfo == nil {
		return
	}
	if r.hasStartedOneBureau {
		return
	}
	userUID := roomUser.UserInfo.Uid
	r.cancelKickScheduleLocked(userUID)
	r.kickSchedules[userUID] = time.AfterFunc(30*time.Second, func() {
		r.Lock()
		currentUser := r.users[userUID]
		shouldStart := false
		shouldDismiss := false
		//需要判断用户是否该踢出
		if !r.hasStartedOneBureau && currentUser != nil && currentUser.UserStatus&enums.Ready == 0 {
			r.kickUserLocked(currentUser, session)
			shouldStart = r.efficacyStartRoom()
			//踢出房间之后，需要判断是否可以解散房间
			shouldDismiss = r.efficacyDismissRoom()
		}
		delete(r.kickSchedules, userUID)
		r.Unlock()
		if shouldStart {
			r.startGame(session, currentUser)
		}
		if shouldDismiss {
			r.DismissRoom(session, enums.DismissNone)
		}
	})
}

func (r *Room) cancelKickScheduleLocked(uid string) {
	if timer, ok := r.kickSchedules[uid]; ok {
		timer.Stop()
		delete(r.kickSchedules, uid)
	}
}

func (r *Room) kickUser(user *proto.RoomUser, session *remote.Session) {
	r.Lock()
	defer r.Unlock()
	r.kickUserLocked(user, session)
}

func (r *Room) kickUserLocked(user *proto.RoomUser, session *remote.Session) {
	if user == nil || user.UserInfo == nil {
		return
	}
	current, ok := r.users[user.UserInfo.Uid]
	if !ok || current != user {
		return
	}
	if r.gameStarted {
		r.GameFrame.OnEventUserOffLine(user, session)
	}
	r.userLeaveRoomNotify([]*proto.RoomUser{user}, session)
	//通知其他人用户离开房间
	r.sendDataLocked(proto.UserLeaveRoomPushData(user), session.GetMsg())
	delete(r.users, user.UserInfo.Uid)
	if r.currentUserCount > 0 {
		r.currentUserCount--
	}
	//关于此用户的定时器停止
	r.cancelKickScheduleLocked(user.UserInfo.Uid)
}

func (r *Room) DismissRoom(session *remote.Session, reason enums.RoomDismissReason) {
	generation := r.roomGeneration.Load()
	r.Lock()
	if !r.beginDismissLocked(generation) {
		r.Unlock()
		return
	}
	// Prepare the result while the room snapshot is stable, then release the
	// room lock before the game callback reads that snapshot through RoomFrame.
	r.RedisService.Delete(r.Id)
	r.cancelAllScheduler()
	r.createHongBaoList()
	r.recordAllDrawResult(session)
	r.Unlock()
	r.destroyRoom(reason, session)
	r.Lock()
	defer r.Unlock()
	r.dismissRoomLocked(session, reason)
}

func (r *Room) beginDismissLocked(generation uint64) bool {
	if r.roomDismissed || generation != r.roomGeneration.Load() {
		return false
	}
	r.roomDismissed = true
	return true
}

func (r *Room) dismissRoomLocked(session *remote.Session, reason enums.RoomDismissReason) {
	//解散 将union当中存储的room信息 删除掉
	//获取并存储房间的数据
	if r.currentUserCount == 0 ||
		reason == enums.UnionOwnerDismiss ||
		r.RoomCreator.CreatorType == enums.UserCreatorType ||
		(reason == enums.UserDismiss && !r.hasFinishedOneBureau) {
		var users []*proto.RoomUser
		for _, v := range r.users {
			users = append(users, v)
		}
		r.userLeaveRoomNotify(users, session)
		r.union.DestroyRoom(r.Id)
		r.sendDataLocked(proto.RoomDismissPushData(reason), session.GetMsg())
	} else {
		// 清除掉线玩家
		r.clearOfflineUserLocked(session)
		if r.currentUserCount == 0 {
			r.union.DestroyRoom(r.Id)
			return
		}
		r.notifyUpdateAllUserInfo(session)
		r.sendDataLocked(proto.RoomDismissPushData(reason), session.GetMsg())
		r.resetRoom(session)
	}
}

func (r *Room) cancelAllScheduler() {
	r.stopAnswerScheduleLocked()
	r.stopStartScheduleLocked()
	//需要将房间所有的任务 都取消掉
	for uid, timer := range r.kickSchedules {
		timer.Stop()
		delete(r.kickSchedules, uid)
	}
}

func (r *Room) stopAnswerScheduleLocked() {
	if r.answerExitSchedule != nil {
		r.answerExitSchedule.Stop()
		r.answerExitSchedule = nil
	}
}

func (r *Room) stopStartScheduleLocked() {
	if r.startSchedulerID != nil {
		r.startSchedulerID.Stop()
		r.startSchedulerID = nil
	}
}

func (r *Room) userReady(uid string, session *remote.Session) {
	if r.gameStarted {
		return
	}
	//1. push用户的座次,修改用户的状态，取消定时任务
	user, ok := r.users[uid]
	if !ok {
		return
	}
	//首局判断玩家积分，如果积分不够则直接踢出游戏
	if r.hasStartedOneBureau && user.UserInfo.Score < r.GameRule.ScoreLowLimit {
		r.sendPopDialogContent(biz.LeaveRoomGoldExceedLimit, []string{user.UserInfo.Uid}, session)
		r.kickUser(user, session)
		return
	}
	if user.UserStatus&enums.Ready == 0 {
		user.UserStatus |= enums.Ready
		user.UserStatus |= enums.Dismiss
	} else {
		return
	}
	r.sendData(proto.UserReadyPushData(user.ChairID), session.GetMsg())
	efficacyStartRoom := r.efficacyStartRoom()
	if efficacyStartRoom {
		r.startGame(session, user)
	} else {
		if r.hasStartedOneBureau {
			return
		}
		if r.gameStarted {
			return
		}
		if r.startSchedulerID != nil {
			return
		}
		start := r.isShouldSchedulerStart()
		if start {
			tick := 10
			r.startSchedulerID = tasks.NewTask("startSchedulerID", 1*time.Second, func() {
				if r.isDismissing() {
					return
				}
				tick--
				if tick >= 0 {
					return
				}
				if !r.isShouldSchedulerStart() {
					return
				}
				//开始游戏
				if r.gameStarted {
					return
				}
				//没准备的玩家转为旁观
				for _, v := range r.users {
					if v.ChairID >= r.chairCount {
						continue
					}
					if v.UserStatus&enums.Ready == 0 {
						r.userChangeSeat(session, v.ChairID, r.getEmptyChairID("", true))
					}
				}
				r.startGame(session, user)
				r.stopStartSchedulerID <- struct{}{}
			})
		}
	}
}

func (r *Room) JoinRoom(session *remote.Session, data *entity.User) *msError.Error {

	return r.UserEntryRoom(session, data)
}

func (r *Room) OtherUserEntryRoomPush(session *remote.Session, uid string) {
	others := make([]string, 0)
	for _, v := range r.users {
		if v.UserInfo.Uid != uid {
			others = append(others, v.UserInfo.Uid)
		}
	}
	user, ok := r.users[uid]
	if ok {
		r.SendData(session.GetMsg(), others, proto.OtherUserEntryRoomPushData(user))
	}
}

func (r *Room) AllUsers() []string {
	r.RLock()
	defer r.RUnlock()
	return r.allUsersLocked()
}

func (r *Room) allUsersLocked() []string {
	users := make([]string, 0)
	for _, v := range r.users {
		if v != nil && v.UserInfo != nil {
			users = append(users, v.UserInfo.Uid)
		}
	}
	return users
}

func (r *Room) getEmptyChairID(uid string, isWatch bool) int {
	r.RLock()
	defer r.RUnlock()
	return r.getEmptyChairIDLocked(uid, isWatch)
}

func (r *Room) getEmptyChairIDLocked(uid string, isWatch bool) int {
	user, ok := r.users[uid]
	if ok {
		return user.ChairID
	}
	isWatch = isWatch || r.hasStartedOneBureau
	used := make(map[int]struct{})
	for _, v := range r.users {
		if v == nil {
			continue
		}
		used[v.ChairID] = struct{}{}
	}
	chairID := 0
	maxChairID := r.chairCount
	if isWatch {
		chairID = r.chairCount
		maxChairID += 20
	}
	for chairID < maxChairID {
		if _, exist := used[chairID]; !exist {
			return chairID
		}
		chairID++
	}
	return -1
}

func (r *Room) IsStartGame() bool {
	r.RLock()
	defer r.RUnlock()
	return r.isStartGameLocked()
}

func (r *Room) isStartGameLocked() bool {
	//房间内准备的人数 已经大于等于 最小开始游戏人数
	userReadyCount := 0
	for _, v := range r.users {
		if v == nil {
			continue
		}
		if v.UserStatus&enums.Ready > 0 {
			userReadyCount++
		}
	}
	if r.GameRule.GameType == enums.ZNMJ {
		if len(r.users) == userReadyCount && userReadyCount >= r.GameRule.MaxPlayerCount {
			return true
		}
	}
	if len(r.users) == userReadyCount && userReadyCount >= r.GameRule.MinPlayerCount {
		return true
	}
	return false
}

func (r *Room) startGame(session *remote.Session, user *proto.RoomUser) {
	if r.gameStarted {
		return
	}
	if r.startSchedulerID != nil {
		r.stopStartSchedulerID <- struct{}{}
	}
	for uid, timer := range r.kickSchedules {
		timer.Stop()
		delete(r.kickSchedules, uid)
	}
	if r.maxBureau > 0 {
		//第一局游戏开局时收取房费
		// 判断联盟是否已经解散
		if r.curBureau == 0 && r.RoomCreator.CreatorType == enums.UnionCreatorType {
			if !r.union.IsOpening() {
				newError := msError.NewError(-1, errors.New("联盟已打烊，无法开始新的牌局"))
				r.sendPopDialogContent(newError, r.getUids(), session)
				r.DismissRoom(session, enums.UnionOwnerDismiss)
				return
			}
		}
		err := r.collectionRoomRentWhenStart(session)
		if err != nil {
			logs.Error("collectionRoomRentWhenStart err=%v", err)
			newError := msError.NewError(-1, errors.New("扣取房费失败，房间已解散"))
			r.sendPopDialogContent(newError, r.getUids(), session)
			r.DismissRoom(session, enums.UnionOwnerDismiss)
			return
		}
		for _, v := range r.users {
			if v == nil || v.UserInfo == nil {
				continue
			}
			if v.ChairID >= r.chairCount {
				continue
			}
			// 记录加入游戏的局数
			value, ok := r.userJoinGameBureau[v.UserInfo.Uid]
			if ok {
				r.userJoinGameBureau[v.UserInfo.Uid] = value + 1
			} else {
				r.userJoinGameBureau[v.UserInfo.Uid] = 1
			}
		}
	}
	r.lastNativeTime = time.Now()
	r.hasStartedOneBureau = true
	r.gameStarted = true
	for _, v := range r.users {
		if v.ChairID < r.chairCount {
			v.UserStatus &= ^enums.Ready
			v.UserStatus |= enums.Playing
		}
	}
	r.GameFrame.OnEventGameStart(user, session)
}

func NewRoom(roomId string, creatorInfo *proto.RoomCreator, rule proto.GameRule, u base.UnionBase, session *remote.Session) (*Room, error) {
	if err := validateRoomConfig(roomId, creatorInfo, rule); err != nil {
		return nil, err
	}
	r := &Room{
		Id:                     roomId,
		unionID:                creatorInfo.UnionID,
		GameRule:               rule,
		users:                  make(map[string]*proto.RoomUser),
		kickSchedules:          make(map[string]*time.Timer),
		union:                  u,
		roomType:               TypeRoomNone,
		chairCount:             rule.MaxPlayerCount,
		maxBureau:              utils.Default(rule.Bureau, 8),
		stopAnswerSchedules:    make(chan struct{}, 1),
		stopStartSchedulerID:   make(chan struct{}, 1),
		userJoinGameBureau:     make(map[string]int),
		userGetHongBaoCountArr: make([]int, 0),
	}
	r.RoomCreator = creatorInfo
	var err error
	r.GameFrame, err = NewGameFrame(rule, r, session)
	if err != nil {
		return nil, err
	}
	go r.stopSchedule()
	return r, nil
}

func validateRoomConfig(roomId string, creatorInfo *proto.RoomCreator, rule proto.GameRule) error {
	if roomId == "" {
		return errors.New("room id is required")
	}
	if creatorInfo == nil {
		return errors.New("room creator is required")
	}
	if rule.MaxPlayerCount < 1 {
		return fmt.Errorf("invalid maxPlayerCount %d", rule.MaxPlayerCount)
	}
	if rule.MinPlayerCount < 1 || rule.MinPlayerCount > rule.MaxPlayerCount {
		return fmt.Errorf("invalid minPlayerCount %d for maxPlayerCount %d", rule.MinPlayerCount, rule.MaxPlayerCount)
	}
	return nil
}
func (r *Room) GetHongBaoList() any {
	r.RLock()
	defer r.RUnlock()
	return append([]int(nil), r.userGetHongBaoCountArr...)
}
func (r *Room) GetUsers() map[string]*proto.RoomUser {
	r.RLock()
	defer r.RUnlock()
	users := make(map[string]*proto.RoomUser, len(r.users))
	for uid, user := range r.users {
		if user == nil || user.UserInfo == nil {
			continue
		}
		users[uid] = user
	}
	return users
}
func (r *Room) GetId() string {
	return r.Id
}
func (r *Room) GameMessageHandle(session *remote.Session, msg []byte) {
	//需要游戏去处理具体的消息
	r.RLock()
	user, ok := r.users[session.GetUid()]
	r.RUnlock()
	if !ok {
		return
	}
	r.RunGameAction(func() {
		r.GameFrame.GameMessageHandle(user, session, msg)
	})
}

func (r *Room) RunGameAction(action func()) {
	if action == nil {
		return
	}
	r.gameActionMu.Lock()
	defer r.gameActionMu.Unlock()
	action()
}

func (r *Room) askForDismiss(session *remote.Session, uid string, exist any) {
	vote, hasVote := exist.(bool)
	if exist != nil && !hasVote {
		return
	}
	r.Lock()
	askUser := r.users[uid]
	if askUser == nil {
		r.Unlock()
		return
	}
	if askUser.UserStatus&enums.Dismiss == 0 || askUser.ChairID < 0 || askUser.ChairID >= r.chairCount {
		r.Unlock()
		if session != nil {
			r.userLeaveRoomRequest(session)
		}
		return
	}
	shouldDismiss := r.askForDismissLocked(session, askUser, exist, vote, hasVote)
	r.Unlock()
	if shouldDismiss {
		r.DismissRoom(session, enums.UserDismiss)
	}
}

func (r *Room) askForDismissLocked(session *remote.Session, askUser *proto.RoomUser, exist any, vote, hasVote bool) bool {
	//所有同意座次的数组
	if (r.askDismiss == nil || len(r.askDismiss) == 0) && hasVote && vote {
		//同意解散
		r.askDismiss = make([]any, r.chairCount)
		r.dismissTick = proto.ExitWaitSecond
		r.answerExitSchedule = tasks.NewTask("answerExitSchedule", 1*time.Second, func() {
			r.onDismissTick(session)
		})
	}
	if r.askDismiss == nil || askUser.ChairID >= len(r.askDismiss) {
		return false
	}
	if r.askDismiss[askUser.ChairID] != nil {
		return false
	}
	r.askDismiss[askUser.ChairID] = exist

	nameArr, avatarArr, onlineArr := dismissUserArrays(r.users, r.chairCount)
	if session != nil {
		for _, v := range r.users {
			if v.UserStatus&enums.Dismiss > 0 && v.ChairID < r.chairCount {
				data := proto.DismissPushData{
					NameArr:    nameArr,
					ChairIDArr: append([]any(nil), r.askDismiss...),
					AskChairId: askUser.ChairID,
					OnlineArr:  onlineArr,
					AvatarArr:  avatarArr,
					Tm:         r.dismissTick,
				}
				r.sendDataOne(proto.AskForDismissPushData(&data), v.UserInfo.Uid, session.GetMsg())
			}
		}
	}
	//不同意直接取消解散申请
	if hasVote && !vote {
		r.stopAnswerScheduleLocked()
		r.askDismiss = nil
	} else if hasVote && vote {
		playUserCount := 0
		agreeDismissCount := 0
		for _, v := range r.users {
			if v.UserStatus&enums.Dismiss > 0 && v.ChairID < r.chairCount {
				playUserCount++
				if r.askDismiss[v.ChairID] != nil {
					agreeDismissCount++
				}
			}
		}
		if playUserCount == agreeDismissCount {
			r.stopAnswerScheduleLocked()
			return true
		}
	}
	return false
}

func (r *Room) onDismissTick(session *remote.Session) {
	r.Lock()
	if r.roomDismissed || r.askDismiss == nil {
		r.stopAnswerScheduleLocked()
		r.Unlock()
		return
	}
	r.dismissTick--
	if r.dismissTick > 0 {
		r.Unlock()
		return
	}
	uids := make([]string, 0, r.chairCount)
	for _, user := range r.users {
		if user != nil && user.UserInfo != nil && user.UserStatus&enums.Dismiss > 0 && user.ChairID >= 0 && user.ChairID < r.chairCount {
			uids = append(uids, user.UserInfo.Uid)
		}
	}
	r.stopAnswerScheduleLocked()
	r.Unlock()
	for _, uid := range uids {
		r.askForDismiss(session, uid, true)
	}
}

func dismissUserArrays(users map[string]*proto.RoomUser, chairCount int) ([]string, []string, []bool) {
	nameArr := make([]string, chairCount)
	avatarArr := make([]string, chairCount)
	onlineArr := make([]bool, chairCount)
	for _, user := range users {
		if user == nil || user.UserInfo == nil || user.ChairID < 0 || user.ChairID >= chairCount || user.UserStatus&enums.Dismiss == 0 {
			continue
		}
		nameArr[user.ChairID] = user.UserInfo.Nickname
		avatarArr[user.ChairID] = user.UserInfo.Avatar
		onlineArr[user.ChairID] = user.UserStatus&enums.Offline == 0
	}
	return nameArr, avatarArr, onlineArr
}

func (r *Room) sendData(data any, msg *stream.Msg) {
	r.SendData(msg, r.AllUsers(), data)
}
func (r *Room) sendDataLocked(data any, msg *stream.Msg) {
	r.SendData(msg, r.allUsersLocked(), data)
}
func (r *Room) sendDataOne(data any, uid string, msg *stream.Msg) {
	r.SendData(msg, []string{uid}, data)
}
func (r *Room) sendDataMany(data any, uids []string, msg *stream.Msg) {
	r.SendData(msg, uids, data)
}
func (r *Room) userLeaveRoomRequest(session *remote.Session) {
	uid := session.GetUid()
	r.RLock()
	user, ok := r.users[uid]
	canLeave := ok && user != nil && user.UserInfo != nil
	if canLeave && r.gameStarted && user.UserStatus&enums.Playing != 0 {
		canLeave = r.GameFrame.IsUserEnableLeave(user.ChairID)
	}
	chairID := -1
	userUID := ""
	if ok && user != nil {
		chairID = user.ChairID
		if user.UserInfo != nil {
			userUID = user.UserInfo.Uid
		}
	}
	r.RUnlock()
	if !ok || user == nil || user.UserInfo == nil {
		return
	}
	if !canLeave {
		r.sendPopDialogContent(biz.CanNotLeaveRoom, []string{userUID}, session)
		r.sendData(proto.UserLeaveRoomResponsePushData(chairID), session.GetMsg())
		return
	}
	r.userLeaveRoom(session)
}

// userChangeSeat 玩家换座位
func (r *Room) userChangeSeat(session *remote.Session, fromChairID int, toChairID int) {
	if fromChairID < 0 || toChairID < 0 {
		return
	}
	r.Lock()
	user, ok := r.users[session.GetUid()]
	if !ok || user == nil || user.UserInfo == nil {
		r.Unlock()
		return
	}
	if user.UserStatus&enums.Playing > 0 {
		//正在游戏不能换座位
		r.Unlock()
		return
	}
	if !r.gameStarted && user.UserStatus&enums.Ready > 0 {
		//如果游戏未开始，且玩家已准备，则重置用户状态
		user.UserStatus = enums.UserStatusNone
	}
	//目标位置有人 不能换座位
	if r.getUserByChairIDLocked(toChairID) != nil {
		r.Unlock()
		return
	}
	//判断用户是否有足够的积分
	if toChairID < r.chairCount && user.UserInfo.Score < r.GameRule.ScoreLowLimit {
		r.Unlock()
		return
	}
	user.ChairID = toChairID
	uid := user.UserInfo.Uid
	r.Unlock()
	//推送给所有用户
	r.sendData(proto.GetUserChangeSeatPush(fromChairID, toChairID, uid), session.GetMsg())
}

func (r *Room) userChat(session *remote.Session, data request.RoomMessageData) {
	r.RLock()
	user, ok := r.users[session.GetUid()]
	if !ok || user == nil {
		r.RUnlock()
		return
	}
	fromChairID := user.ChairID
	r.RUnlock()
	r.SendDataAll(session.GetMsg(), proto.UserChatPushData(fromChairID, data.ToChairID, data.Msg.Value()))

}

func (r *Room) userLeaveRoom(session *remote.Session) {
	user, ok := r.users[session.GetUid()]
	if !ok {
		log.Printf("leave room fail,roomId:%s, not this user: %s\n", r.Id, session.GetUid())
		return
	}
	//推送所有人 此用户离开
	r.sendData(proto.UserLeaveRoomResponsePushData(user.ChairID), session.GetMsg())
	if r.gameStarted && (user.UserStatus&enums.Playing != 0) {
		//正在游戏途中 离开
		if r.GameFrame.IsUserEnableLeave(user.ChairID) {
			r.kickUser(user, session)
		} else {
			//离线了
			user.UserStatus |= enums.Offline
			if r.roomType != Hundred {
				r.sendData(proto.UserOffLinePushData(user.ChairID), session.GetMsg())
			}
			r.GameFrame.OnEventUserOffLine(user, session)
		}
	} else {
		r.kickUser(user, session)
	}
	if r.efficacyStartRoom() {
		r.startGame(session, user)
	}
	//判断房间是否需要解散
	if r.efficacyDismissRoom() {
		r.DismissRoom(session, enums.DismissNone)
	}
}

func (r *Room) efficacyDismissRoom() bool {
	if r.roomDismissed {
		return false
	}
	return r.currentUserCount == 0
}

func (r *Room) getUserByChairID(chairID int) *proto.RoomUser {
	r.RLock()
	defer r.RUnlock()
	return r.getUserByChairIDLocked(chairID)
}

func (r *Room) getUserByChairIDLocked(chairID int) *proto.RoomUser {
	for _, value := range r.users {
		if value != nil && value.ChairID == chairID {
			return value
		}
	}
	return nil
}

func (r *Room) sendPopDialogContent(code *msError.Error, uids []string, session *remote.Session) {
	r.sendDataMany(proto.PopDialogContentPushData(code), uids, session.GetMsg())
}

func (r *Room) userLeaveRoomNotify(users []*proto.RoomUser, session *remote.Session) {
	for _, user := range users {
		err := r.UserService.UpdateUserRoomId(context.Background(), user.UserInfo.Uid, "")
		if err != nil {
			logs.Error("UpdateUserRoomId err : %v", err)
			return
		}
		session.Put("roomId", "", stream.Single)
		//推送用户数据变化
		r.sendDataOne(proto.UpdateUserInfoPush(map[string]any{
			"roomID": "",
		}), user.UserInfo.Uid, session.GetMsg())
	}
}

func (r *Room) efficacyStartRoom() bool {
	if r.roomDismissed || r.gameStarted {
		return false
	}
	readyCount := 0
	userCount := 0
	for _, v := range r.users {
		if v.ChairID < r.chairCount {
			userCount++
			if v.UserStatus&enums.Ready > 0 {
				readyCount++
			}
		}
	}
	return (userCount == readyCount && userCount >= r.GameRule.MinPlayerCount) || readyCount == r.GameRule.MaxPlayerCount
}

func (r *Room) isShouldSchedulerStart() bool {
	if r.gameStarted {
		return false
	}
	if r.hasStartedOneBureau {
		return false
	}
	if r.roomDismissed {
		return false
	}
	readyCount := 0
	for _, v := range r.users {
		if v.ChairID < r.chairCount {
			if v.UserStatus&enums.Ready > 0 {
				readyCount++
			}
		}
	}
	return readyCount >= 4 && readyCount >= r.GameRule.MinPlayerCount
}

// 正在解散中
func (r *Room) isDismissing() bool {
	r.RLock()
	defer r.RUnlock()
	return r.askDismiss != nil && len(r.askDismiss) > 0
}

func (r *Room) CanEnter() bool {
	r.RLock()
	defer r.RUnlock()
	return r.canEnterLocked()
}

func (r *Room) canEnterLocked() bool {
	hasEmpty := r.hasEmptyChairLocked()
	canWatch := r.GameRule.CanWatch
	canEnter := r.GameRule.CanEnter && (r.GameRule.GameType != enums.PDK)
	if r.hasStartedOneBureau {
		return canEnter && (hasEmpty || (canWatch && r.currentUserCount < 20))
	}
	return hasEmpty || (canWatch && r.currentUserCount < 20)
}

func (r *Room) HasEmptyChair() bool {
	r.RLock()
	defer r.RUnlock()
	return r.hasEmptyChairLocked()
}

func (r *Room) hasEmptyChairLocked() bool {
	seatCount := 0
	for _, v := range r.users {
		if v == nil {
			continue
		}
		if v.ChairID < 0 || v.ChairID >= r.chairCount {
			continue
		}
		seatCount++
	}
	return seatCount < r.chairCount
}

func (r *Room) checkEntryRoom(userInfo *proto.UserInfo) *msError.Error {
	if r.RoomCreator.CreatorType == enums.UserCreatorType {
		//普通房间
		if r.GameRule.PayType == proto.MyPay {
			//检查钻石是否足够
			if userInfo.Uid == r.RoomCreator.Uid {
				if userInfo.Gold < int64(r.GameRule.PayDiamond) {
					return biz.NotEnoughGold
				}
			}
		} else {
			if userInfo.Gold < int64(r.GameRule.PayDiamond) {
				return biz.NotEnoughGold
			}
		}
	}
	return nil
}

func (r *Room) sendDataExceptUid(data any, uid string, msg *stream.Msg) {
	var uids []string
	for _, v := range r.users {
		if v == nil || v.UserInfo == nil {
			continue
		}
		if v.UserInfo.Uid == uid {
			continue
		}
		uids = append(uids, v.UserInfo.Uid)
	}
	r.SendData(msg, uids, data)
}

func (r *Room) destroyRoom(reason enums.RoomDismissReason, session *remote.Session) {
	r.GameFrame.OnEventRoomDismiss(reason, session)
}

func (r *Room) clearOfflineUserLocked(session *remote.Session) {
	for _, v := range r.users {
		if v != nil && v.UserStatus&enums.Offline != 0 {
			r.kickUserLocked(v, session)
		}
	}
}

func (r *Room) notifyUpdateAllUserInfo(session *remote.Session) {
	for _, v := range r.users {
		if v == nil || v.UserInfo == nil || v.ChairID < 0 || v.ChairID >= r.chairCount {
			continue
		}
		r.sendData(proto.UserInfoChangePushData(v.UserInfo), session.GetMsg())
	}
}

func (r *Room) recordGameResult(dataArr []*proto.EndData, session *remote.Session) {
	if dataArr == nil || len(dataArr) == 0 {
		return
	}
	var updateUserArr []*entity.User
	for _, v := range dataArr {
		user := r.users[v.Uid]
		if r.RoomCreator.CreatorType == enums.UnionCreatorType {
			//计算最终获得的金币数量 并进行存储
			updateUser := r.UserService.UpdateUserDataScoreInc(v.Uid, r.RoomCreator.UnionID, v.Score)
			if updateUser != nil {
				updateUserArr = append(updateUserArr, updateUser)
			}
		} else {
			userInfo := &proto.UserInfo{
				Uid:   user.UserInfo.Uid,
				Score: user.UserInfo.Score + v.Score,
			}
			r.updateRoomUserInfo(userInfo, false, session)
		}
		user.WinScore = user.WinScore + v.Score
	}
	if len(updateUserArr) > 0 {
		var scoreChangeRecordArr []*entity.UserScoreChangeRecord
		for _, v := range updateUserArr {
			_, ok := r.users[v.Uid]
			if ok {
				r.updateRoomUserInfo(proto.BuildGameRoomUserInfoWithUnion(v, r.RoomCreator.UnionID, session.GetMsg().ConnectorId), false, session)
			}
			r.updateUserDataNotify(map[string]any{
				"unionInfo": v.UnionInfo,
			}, session)
			if r.RoomCreator.CreatorType != enums.UnionCreatorType {
				continue
			}
			var data *proto.EndData
			for _, e := range dataArr {
				if e.Uid == v.Uid {
					data = e
					break
				}
			}
			var newUnionInfo *entity.UnionInfo
			for _, u := range v.UnionInfo {
				if u.UnionID == r.RoomCreator.UnionID {
					newUnionInfo = u
					break
				}
			}
			var describe string
			if data.Score > 0 {
				describe = "赢分" + strconv.Itoa(data.Score)
			} else {
				describe = "输分" + strconv.Itoa(-data.Score)
			}
			scoreChangeRecordArr = append(scoreChangeRecordArr, &entity.UserScoreChangeRecord{
				CreateTime:       time.Now().UnixMilli(),
				Uid:              v.Uid,
				Nickname:         v.Nickname,
				UnionID:          r.RoomCreator.UnionID,
				ChangeCount:      int64(data.Score),
				LeftCount:        int64(newUnionInfo.Score),
				LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
				ChangeType:       enums.GameWin,
				Describe:         describe,
			})
		}
		if len(scoreChangeRecordArr) > 0 {
			_ = r.UserService.SaveUserScoreChangeRecordList(scoreChangeRecordArr)
		}
	}
}

func (r *Room) updateRoomUserInfo(userInfo *proto.UserInfo, notify bool, session *remote.Session) {
	user, ok := r.users[userInfo.Uid]
	if !ok {
		return
	}
	if userInfo.Score > 0 {
		user.UserInfo.Score = userInfo.Score
	}
	if userInfo.Avatar != "" {
		user.UserInfo.Avatar = userInfo.Avatar
	}
	if userInfo.Gold > 0 {
		user.UserInfo.Gold = userInfo.Gold
	}
	if notify {
		r.sendData(proto.UserInfoChangePushData(user.UserInfo), session.GetMsg())
	}
}

func (r *Room) updateUserDataNotify(data map[string]any, session *remote.Session) {
	r.sendData(proto.UpdateUserInfoPush(data), session.GetMsg())
}

// calculateRebateWhenStart 计算返利数量
func (r *Room) calculateRebateWhenStart(session *remote.Session) {
	if r.RoomCreator.CreatorType == enums.UnionCreatorType {
		return
	}
	roomPayRule := r.GameRule.RoomPayRule
	if roomPayRule.EveryFixedScore <= 0 {
		return
	}
	rebateList := make(map[string]int)
	for key, v := range r.users {
		if v.ChairID >= r.chairCount {
			rebateList[key] = -1
			continue
		}
		if utils.Contains(r.alreadyCostUserUidArr, v.UserInfo.Uid) {
			rebateList[key] = -1
			continue
		}
		rebateList[key] = roomPayRule.EveryFixedScore
		rebateList[key] += roomPayRule.EveryAgentFixedScore
	}
	var totalRebateCount float64
	var scoreChangeRecordArr []*entity.UserScoreChangeRecord
	for key, v := range r.users {
		if rebateList[key] == -1 {
			continue
		}
		count := int(math.Floor(float64(rebateList[key]*100)) / 100)
		if count <= 0 {
			continue
		}
		newUserData := r.UserService.UpdateUserDataScoreInc(key, r.RoomCreator.UnionID, int(-count))
		if newUserData == nil {
			logs.Error("[Room] deduct fixed room fee failed uid:%s unionID:%d", key, r.RoomCreator.UnionID)
			continue
		}
		r.updateRoomUserInfo(proto.BuildGameRoomUserInfoWithUnion(newUserData, r.RoomCreator.UnionID, session.GetMsg().ConnectorId), false, session)
		r.updateUserDataNotify(map[string]any{
			"unionInfo": newUserData.UnionInfo,
		}, session)
		//存储分数变化记录
		var newUnionInfo *entity.UnionInfo
		for _, v := range newUserData.UnionInfo {
			if v.UnionID == r.RoomCreator.UnionID {
				newUnionInfo = v
				break
			}
		}
		if newUnionInfo == nil {
			logs.Error("[Room] deducted user union info missing uid:%s unionID:%d", key, r.RoomCreator.UnionID)
			continue
		}
		scoreChangeRecordArr = append(scoreChangeRecordArr, &entity.UserScoreChangeRecord{
			CreateTime:       time.Now().UnixMilli(),
			Uid:              newUserData.Uid,
			Nickname:         newUserData.Nickname,
			UnionID:          r.RoomCreator.UnionID,
			ChangeCount:      int64(-count),
			LeftCount:        int64(newUnionInfo.Score),
			LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
			ChangeType:       enums.GameStartUnionChou,
			Describe:         fmt.Sprintf("抽取房费%d", count),
		})
		totalRebateCount += float64(count) - float64(roomPayRule.EveryAgentFixedScore)
		// 计算代理固定返利
		if roomPayRule.EveryAgentFixedScore > 0 {
			r.execRebate(
				r.RoomCreator.UnionID,
				r.Id,
				r.GameRule.GameType, v.UserInfo,
				nil,
				"",
				key,
				roomPayRule.EveryAgentFixedScore,
				false,
				true,
				session,
			)
		}
	}
	if len(scoreChangeRecordArr) > 0 {
		r.UserService.SaveUserScoreChangeRecordList(scoreChangeRecordArr)
	}
	totalRebateCount = math.Floor(totalRebateCount*100) / 100
	if totalRebateCount > 0 {
		unionOwnerUid := r.union.GetOwnerUid()
		saveData := bson.M{
			"$inc": bson.M{
				"unionInfo.$.safeScore":   totalRebateCount,
				"unionInfo.$.todayRebate": totalRebateCount,
				"unionInfo.$.totalRebate": totalRebateCount,
			},
		}
		newUserData := r.UserService.UpdateUserData(bson.M{"uid": unionOwnerUid, "unionInfo.unionID": r.RoomCreator.UnionID}, saveData)
		if newUserData == nil {
			logs.Error("[Room] update union owner rebate failed uid:%s unionID:%d", unionOwnerUid, r.RoomCreator.UnionID)
			return
		}
		if newUserData.FrontendId != "" {
			r.UserService.UpdateUserDataNotify(newUserData.Uid, newUserData.FrontendId, map[string]any{
				"unionInfo": newUserData.UnionInfo,
			}, session)
		}
		//添加记录
		r.UserService.SaveUserRebateRecord(&entity.UserRebateRecord{
			CreateTime: time.Now().UnixMilli(),
			Uid:        unionOwnerUid,
			RoomID:     r.Id,
			UnionID:    r.RoomCreator.UnionID,
			GameType:   int(r.GameRule.GameType),
			PlayerUid:  "",
			TotalCount: int(totalRebateCount),
			GainCount:  int(totalRebateCount),
			Start:      true,
		})
	}
}
func (r *Room) execRebate(unionID int64, roomId string, gameType enums.GameType, userInfo *proto.UserInfo, lowPartnerUnionInfo *entity.UnionInfo, lowUid string, spreaderID string, count int, bigWin bool, isOneDraw bool, session *remote.Session) {
	if spreaderID == "" {
		return
	}
	var lowPartnerRebateRate float64
	if lowPartnerUnionInfo != nil {
		lowPartnerRebateRate = lowPartnerUnionInfo.RebateRate
	}
	userData, err := r.UserService.FindUserByUid(context.TODO(), spreaderID)
	if err != nil {
		logs.Error("FindUserByUid err : %v", err)
		return
	}
	if userData == nil {
		logs.Error("FindUserByUid returned nil uid:%s", spreaderID)
		return
	}
	var unionInfo *entity.UnionInfo
	for _, v := range userData.UnionInfo {
		if v.UnionID == unionID {
			unionInfo = v
		}
	}
	if unionInfo == nil {
		return
	}
	// 第一步：计算初始分数
	getScore := float64(count) * (unionInfo.RebateRate - lowPartnerRebateRate)
	// 第二步：向下取整到两位小数
	getScore = math.Floor(getScore*100) / 100
	if getScore <= 0 {
		return
	}
	if getScore > 0 {
		saveData := bson.M{
			"$inc": bson.M{
				"unionInfo.$.safeScore":   getScore,
				"unionInfo.$.todayRebate": getScore,
				"unionInfo.$.totalRebate": getScore,
			},
		}
		if !isOneDraw {
			saveData["$inc"].(bson.M)["unionInfo.$.memberTodayDraw"] = 1
			if bigWin {
				saveData["$inc"].(bson.M)["unionInfo.$.memberTodayBigWinDraw"] = 1
			}
		}
		matchData := bson.M{"unionInfo.unionID": unionID, "uid": spreaderID}
		newUserData := r.UserService.UpdateUserData(matchData, saveData)
		if newUserData == nil {
			logs.Error("UpdateUserData rebate failed uid:%s unionID:%d", spreaderID, unionID)
			return
		}
		r.updateUserDataNotify(map[string]any{"unionInfo": newUserData.UnionInfo}, session)
		// 记录下级玩家贡献的的返利数
		if lowUid != "" {
			matchData = bson.M{"unionInfo.unionID": unionID, "uid": lowUid}
			saveData = bson.M{"$inc": bson.M{"unionInfo.$.todayProvideRebate": getScore}}
			r.UserService.UpdateUserData(matchData, saveData)
		}
		// 添加记录
		createData := &entity.UserRebateRecord{
			CreateTime: time.Now().UnixMilli(),
			Uid:        spreaderID,
			RoomID:     roomId,
			GameType:   int(gameType),
			UnionID:    unionID,
			PlayerUid:  userInfo.Uid,
			TotalCount: count,
			GainCount:  int(getScore),
			Start:      false,
		}
		_ = r.UserService.SaveUserRebateRecord(createData)
	} else if !isOneDraw {
		matchData := bson.M{"unionInfo.unionID": unionID, "uid": spreaderID}
		saveData := bson.M{"$inc": bson.M{"unionInfo.$.memberTodayDraw": 1}}
		if bigWin {
			saveData["$inc"].(bson.M)["unionInfo.$.memberTodayBigWinDraw"] = 1
		}
		r.UserService.UpdateUserData(matchData, saveData)
	}
	if unionInfo.SpreaderID != "" || unionInfo.RebateRate >= 1 {
		return
	}
	r.execRebate(unionInfo.UnionID, roomId, gameType, userInfo, unionInfo, spreaderID, unionInfo.SpreaderID, count, bigWin, isOneDraw, session)
}

func (r *Room) recordOneDrawResult(dataArr []*proto.EndData, session *remote.Session) {
	if r.RoomCreator.CreatorType != enums.UnionCreatorType {
		return
	}
	if r.GameRule.RoomPayRule.RebateType != enums.One {
		return
	}
	dataList := make(map[string]*proto.RoomUser)
	for _, v := range dataArr {
		dataList[v.Uid] = &proto.RoomUser{
			Uid:      v.Uid,
			WinScore: v.Score,
		}
	}
	rebateList := r.calculateRebate(dataList)
	avgRebateCount := 0
	if !r.GameRule.RoomPayRule.IsAvg {
		totalRebateCount := 0
		for _, v := range rebateList {
			totalRebateCount += v
		}
		// 计算参与游戏的有效玩家数量
		validUserCount := len(dataArr)
		if validUserCount == 0 {
			avgRebateCount = 0
		} else {
			avgRebateCount = int(math.Floor(((float64(totalRebateCount) / float64(validUserCount)) * 100) / 100))
		}
	}
	for uid, user := range r.users {
		_, ok := dataList[uid]
		if !ok {
			continue
		}
		if user.ChairID >= r.chairCount {
			continue
		}
		rebateCount := avgRebateCount
		if !r.GameRule.RoomPayRule.IsAvg {
			rebateCount = rebateList[uid]
		}
		if rebateCount <= 0 {
			continue
		}
		var saveData bson.M
		if rebateList[uid] > 0 {
			count := int(math.Floor(float64(rebateList[uid]) * 100 / 100))
			saveData = bson.M{"$inc": bson.M{"unionInfo.$.score": -count}}
		}
		matchData := bson.M{"unionInfo.unionID": r.RoomCreator.UnionID, "uid": uid}
		newUserData := r.UserService.UpdateUserData(matchData, saveData)
		if newUserData == nil {
			logs.Error("UpdateUserData one-draw rebate failed uid:%s unionID:%d", uid, r.RoomCreator.UnionID)
			continue
		}
		r.updateUserDataNotify(map[string]any{"unionInfo": newUserData.UnionInfo}, session)
		r.updateRoomUserInfo(proto.BuildGameRoomUserInfoWithUnion(newUserData, r.RoomCreator.UnionID, session.GetMsg().ConnectorId), false, session)
		r.execRebate(r.RoomCreator.UnionID, r.Id, r.GameRule.GameType, user.UserInfo, nil, "", uid, rebateCount, false, true, session)
	}
}

func (r *Room) clearNonSatisfiedConditionsUser(session *remote.Session) {
	if r.RoomCreator.CreatorType != enums.UnionCreatorType {
		return
	}
	var kickUidArr []string
	var kickChairIDArr []int
	for _, user := range r.users {
		if user.ChairID >= r.chairCount {
			continue
		}
		if user.UserInfo.Score < r.GameRule.ScoreDismissLimit {
			if r.GameRule.GameType == enums.PDK || r.GameRule.GameType == enums.ZNMJ {
				r.DismissRoom(session, enums.UserDismiss)
			} else {
				if r.GameRule.CanEnter && r.GameRule.CanWatch {
					r.userChangeSeat(session, user.ChairID, r.getEmptyChairID("", true))
				} else {
					kickUidArr = append(kickUidArr, user.UserInfo.Uid)
					kickChairIDArr = append(kickChairIDArr, user.ChairID)
				}
				_, ok := r.clearUserArr[user.UserInfo.Uid]
				if ok {
					r.clearUserArr[user.UserInfo.Uid].WinScore += user.WinScore
				} else {
					r.clearUserArr[user.UserInfo.Uid] = &entity.GameUser{
						Uid:        user.UserInfo.Uid,
						Score:      int64(user.WinScore),
						Avatar:     user.UserInfo.Avatar,
						Nickname:   user.UserInfo.Nickname,
						SpreaderID: user.UserInfo.SpreaderID,
					}
				}
			}
		}
	}
	if len(kickUidArr) > 0 {
		r.sendPopDialogContent(biz.LeaveRoomGoldNotEnoughLimit, kickUidArr, session)
		for _, v := range kickUidArr {
			r.kickUser(r.users[v], session)
		}
	}
}

// calculateRebate 计算返利数量
func (r *Room) calculateRebate(dataList map[string]*proto.RoomUser) map[string]int {
	roomPayRule := r.GameRule.RoomPayRule
	// 大赢家支付
	bigWinUidArr := r.getBinWinUidArr(dataList)

	rebateList := make(map[string]int)
	for _, uid := range bigWinUidArr {
		user := dataList[uid]
		winScore := user.WinScore
		if winScore >= roomPayRule.FixedMinWinScore {
			count := roomPayRule.FixedScore
			rebateList[uid] = rebateList[uid] + count
			winScore -= count
		}
		if winScore >= roomPayRule.PercentMinWinScore {
			count := int(math.Floor((float64(roomPayRule.PercentScore) / float64(100)) * float64(winScore)))
			rebateList[uid] = rebateList[uid] + count
		}
	}
	return rebateList
}

func (r *Room) getBinWinUidArr(dataList map[string]*proto.RoomUser) []string {
	var userWinScoreArr []*proto.RoomUser
	for uid, user := range dataList {
		if !utils.Contains(r.alreadyCostUserUidArr, uid) {
			continue
		}
		if user.WinScore <= 0 {
			continue
		}
		userWinScoreArr = append(userWinScoreArr, user)
	}
	//排序
	sort.Slice(userWinScoreArr, func(i, j int) bool {
		return userWinScoreArr[i].WinScore > userWinScoreArr[j].WinScore
	})
	roomPayRule := r.GameRule.RoomPayRule
	bigWinCount := 100
	if roomPayRule.BigWinCount != -1 {
		bigWinCount = roomPayRule.BigWinCount
	}
	var bigWinUidArr []string
	bigWinScore := userWinScoreArr[0].WinScore
	for _, v := range userWinScoreArr {
		if v.WinScore <= 0 {
			continue
		}
		if bigWinCount <= 0 && v.WinScore != bigWinScore {
			break
		}
		bigWinUidArr = append(bigWinUidArr, v.Uid)
		bigWinCount--
	}
	return bigWinUidArr
}

func (r *Room) stopSchedule() {
	for {
		select {
		case <-r.stopAnswerSchedules:
			if r.answerExitSchedule != nil {
				r.answerExitSchedule.Stop()
				r.answerExitSchedule = nil
			}
		case <-r.stopStartSchedulerID:
			if r.startSchedulerID != nil {
				r.startSchedulerID.Stop()
				r.startSchedulerID = nil
			}

		}
	}
}

func (r *Room) GetRoomInfo() *proto.RoomInfo {
	r.RLock()
	defer r.RUnlock()
	if r.roomDismissed {
		return nil
	}
	var roomUserInfoArr []*proto.UserRoomData
	for _, v := range r.users {
		if v == nil || v.UserInfo == nil {
			continue
		}
		if v.ChairID >= r.chairCount {
			continue
		}
		roomUserInfoArr = append(roomUserInfoArr, &proto.UserRoomData{
			WinScore: v.WinScore,
			Avatar:   v.UserInfo.Avatar,
			Nickname: v.UserInfo.Nickname,
		})
	}
	return &proto.RoomInfo{
		RoomID:          r.Id,
		GameRule:        r.GameRule,
		GameStarted:     r.gameStarted,
		CurBureau:       r.curBureau,
		RoomUserInfoArr: roomUserInfoArr,
	}
}

func (r *Room) IsUserInRoom(uid string) bool {
	r.RLock()
	defer r.RUnlock()
	if r.roomDismissed {
		return false
	}
	return r.users[uid] != nil
}

func (r *Room) UpdateLotteryInfo(status *entity.ResultLotteryInfo) {
	r.resultLotteryInfo = status
}

func (r *Room) createHongBaoList() {
	status := r.resultLotteryInfo != nil && r.resultLotteryInfo.Status
	var arr []int
	var countArr []int
	if r.resultLotteryInfo != nil && r.resultLotteryInfo.CountArr != nil && len(r.resultLotteryInfo.CountArr) == 6 {
		countArr = r.resultLotteryInfo.CountArr
	} else {
		countArr = []int{1, 2, 8, 18, 88, 888}
	}
	var rateArr []float64
	if r.resultLotteryInfo != nil && r.resultLotteryInfo.RateArr != nil && len(r.resultLotteryInfo.RateArr) == 6 {
		rateArr = r.resultLotteryInfo.RateArr
	} else {
		rateArr = []float64{0.34, 0.60, 0.05, 0.009, 0.001, 0}
	}
	for i := 0; i < 10; i++ {
		if !status {
			arr = append(arr, -1)
			continue
		}
		user := r.getUserByChairIDLocked(i)
		if user == nil {
			arr = append(arr, -1)
			continue
		}
		uid := user.UserInfo.Uid
		if r.userJoinGameBureau[uid] > 0 && r.userJoinGameBureau[uid] >= r.GameRule.Bureau-3 {
			count := countArr[0]
			rands := rand.Float64()
			for index, rateValue := range rateArr {
				if rands < rateValue {
					count = countArr[index]
					break
				} else {
					rands -= rateValue
				}
			}
			arr = append(arr, count)
		} else {
			arr = append(arr, -1)
		}
	}
	r.userGetHongBaoCountArr = arr
}

func (r *Room) getUids() []string {
	var uids []string
	for _, v := range r.users {
		if v == nil || v.UserInfo == nil {
			continue
		}
		uids = append(uids, v.UserInfo.Uid)
	}
	return uids
}

func (r *Room) collectionRoomRentWhenStart(session *remote.Session) error {
	if r.RoomCreator.CreatorType == enums.UserCreatorType {
		var newUserDataArr []*entity.User
		if r.GameRule.PayType == enums.AAZhiFu {
			for key, _ := range r.users {
				savaData := bson.M{
					"$inc": bson.M{
						"gold": -r.GameRule.PayDiamond,
					},
				}
				matchData := bson.M{"uid": key}
				updated := r.UserService.UpdateUserData(matchData, savaData)
				if updated == nil {
					return biz.SqlError
				}
				newUserDataArr = append(newUserDataArr, updated)
			}
			for _, updateUserData := range newUserDataArr {
				if updateUserData.FrontendId != "" {
					r.UserService.UpdateUserDataNotify(updateUserData.Uid, updateUserData.FrontendId, map[string]any{"gold": updateUserData.Gold}, session)
				}
			}
		} else if r.GameRule.PayType == enums.MyPay {
			newUserData := r.UserService.UpdateUserData(bson.M{"uid": r.RoomCreator.Uid}, bson.M{"$inc": bson.M{"gold": -r.GameRule.PayDiamond}})
			if newUserData == nil {
				return biz.NotEnoughGold
			}
			if newUserData.FrontendId != "" {
				r.UserService.UpdateUserDataNotify(newUserData.Uid, newUserData.FrontendId, map[string]any{"gold": newUserData.Gold}, session)
			}
		} else {
			var costUserCount int
			for uid, user := range r.users {
				if user.ChairID >= r.chairCount {
					continue
				}
				if utils.IndexOf(r.alreadyCostUserUidArr, uid) != -1 {
					continue
				}
				costUserCount++
			}
			if costUserCount == 0 {
				return nil
			}
			payDiamondCount := proto.OneUserDiamondCount(r.GameRule.Bureau, r.GameRule.GameType) * costUserCount
			if payDiamondCount <= 0 {
				logs.Error("unsupported room rent rule gameType:%d bureau:%d", r.GameRule.GameType, r.GameRule.Bureau)
				return biz.RequestDataError
			}
			matchData := bson.M{
				"uid": r.union.GetOwnerUid(),
			}
			// 首局开局时检测金币是否足够，不够则解散房间
			if r.curBureau == 0 {
				matchData["gold"] = bson.M{"$gte": payDiamondCount}
			}
			userData := r.UserService.UpdateUserData(matchData, bson.M{"$inc": bson.M{"gold": -payDiamondCount}})
			if userData == nil {
				return biz.NotEnoughGold
			} else {
				if userData.FrontendId != "" {
					r.UserService.UpdateUserDataNotify(userData.Uid, userData.FrontendId, map[string]any{"gold": userData.Gold}, session)
				}
			}
		}
	}
	return nil
}

func (r *Room) recordAllDrawResult(session *remote.Session) {
	if !r.hasFinishedOneBureau {
		return
	}
	// 俱乐部模式下记录游戏数据
	if r.RoomCreator.CreatorType == enums.UnionCreatorType {
		var rebateList = make(map[string]int)
		var avgRebateCount int64
		allPlayedUserArr := r.allPlayedUsersLocked()
		var invalidRebateUserArr []string
		if r.GameRule.RoomPayRule.RebateType != enums.One {
			rebateList = r.calculateRebate(r.users)
			if r.GameRule.RoomPayRule.IsAvg {
				var totalRebateCount int64
				for _, value := range rebateList {
					totalRebateCount += int64(value)
				}
				// 计算参与游戏的有效玩家数量
				var validUserCount int
				for key, _ := range allPlayedUserArr {
					if r.userJoinGameBureau[key] > 0 && r.userJoinGameBureau[key] >= r.GameRule.Bureau-3 {
						validUserCount++
					} else {
						invalidRebateUserArr = append(invalidRebateUserArr, key)
					}
				}
				if validUserCount == 0 {
					avgRebateCount = 0
				} else {
					avgRebate := float64(totalRebateCount) / float64(validUserCount)
					avgRebateCount = int64(math.Floor(avgRebate*100) / 100)
				}
			}
		}
		bigWinUidArr := r.getBinWinUidArr(r.users)
		var scoreChangeRecordArr []*entity.UserScoreChangeRecord
		newTime := time.Now().UnixMilli()
		for key, user := range allPlayedUserArr {
			saveData := bson.M{
				"$inc": bson.M{
					"unionInfo.$.todayDraw": 1,
					"unionInfo.$.totalDraw": 1,
					"unionInfo.$.weekDraw":  1,
					"unionInfo.$.todayWin":  user.WinScore,
				},
			}
			if utils.IndexOf(bigWinUidArr, key) != -1 {
				saveData["$inc"].(bson.M)["unionInfo.$.todayBigWinDraw"] = 1
			}
			rebateCount := avgRebateCount
			if !r.GameRule.RoomPayRule.IsAvg {
				rebateCount = int64(rebateList[key])
			}
			// 在avg模式下，过滤无效用户的返利
			if r.GameRule.RoomPayRule.IsAvg && utils.IndexOf(invalidRebateUserArr, key) != -1 {
				rebateCount = 0
			}
			if rebateList[key] > 0 {
				count := math.Floor(float64(rebateList[key]*100)) / 100
				saveData["$inc"].(bson.M)["unionInfo.$.score"] = -count
			}
			// 记录红包数量
			hongBaoCount := r.userGetHongBaoCountArr[user.ChairID]
			if hongBaoCount > 0 {
				i := saveData["$inc"].(bson.M)["unionInfo.$.score"]
				if i == nil {
					i = 0
				}
				saveData["$inc"].(bson.M)["unionInfo.$.score"] = i.(int) + hongBaoCount
			}
			newUserData := r.UserService.UpdateUserData(bson.M{"unionInfo.unionID": r.RoomCreator.UnionID, "uid": key}, saveData)
			if newUserData == nil {
				logs.Error("[Room] update game result score failed uid:%s unionID:%d", key, r.RoomCreator.UnionID)
				continue
			}
			if newUserData.FrontendId != "" {
				r.UserService.UpdateUserDataNotify(newUserData.Uid, newUserData.FrontendId, map[string]any{
					"unionInfo": newUserData.UnionInfo,
				}, session)
			}
			var newUnionInfo *entity.UnionInfo
			for _, v := range newUserData.UnionInfo {
				if v.UnionID == r.RoomCreator.UnionID {
					newUnionInfo = v
					break
				}
			}
			if newUnionInfo == nil {
				logs.Error("[Room] game result union info missing uid:%s unionID:%d", key, r.RoomCreator.UnionID)
				continue
			}
			if rebateCount > 0 {
				scoreChangeRecordArr = append(scoreChangeRecordArr, &entity.UserScoreChangeRecord{
					Uid:              newUserData.Uid,
					Nickname:         newUserData.Nickname,
					UnionID:          r.RoomCreator.UnionID,
					ChangeCount:      -rebateCount,
					LeftCount:        int64(newUnionInfo.Score),
					LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
					ChangeType:       enums.GameWinChou,
					Describe:         fmt.Sprintf("赢家抽分%d", rebateCount),
					CreateTime:       newTime,
				})
			}
			if hongBaoCount > 0 {
				scoreChangeRecordArr = append(scoreChangeRecordArr, &entity.UserScoreChangeRecord{
					Uid:              newUserData.Uid,
					Nickname:         newUserData.Nickname,
					UnionID:          r.RoomCreator.UnionID,
					ChangeCount:      -rebateCount,
					LeftCount:        int64(newUnionInfo.Score),
					LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
					ChangeType:       enums.ScoreChangeNone,
					Describe:         fmt.Sprintf("红包抽奖%d", hongBaoCount),
					CreateTime:       newTime,
				})
			}
			r.updateRoomUserInfo(proto.BuildGameRoomUserInfoWithUnion(newUserData, r.RoomCreator.UnionID, session.GetMsg().ConnectorId), false, session)
			r.execRebate(r.RoomCreator.UnionID, r.Id, r.GameRule.GameType, user.UserInfo, nil, "", key, int(rebateCount), utils.IndexOf(bigWinUidArr, key) != -1, false, session)
		}
		if len(scoreChangeRecordArr) > 0 {
			r.UserService.SaveUserScoreChangeRecordList(scoreChangeRecordArr)
		}

	}
	var gameVideoRecord *entity.GameVideoRecord
	// 记录录像
	gameVideoData := r.GameFrame.GetGameVideoData()
	if gameVideoData != nil {
		marshal, _ := json.Marshal(gameVideoData)
		savaData := &entity.GameVideoRecord{
			RoomID:     r.Id,
			GmeType:    int(r.GameRule.GameType),
			Detail:     string(marshal),
			CreateTime: time.Now().UnixMilli(),
		}
		r.UserService.SaveGameVideoRecord(savaData)
		gameVideoRecord = savaData
	}
	var userList []*entity.GameUser
	// 记录游戏数据
	for key, v := range r.users {
		if utils.IndexOf(r.alreadyCostUserUidArr, key) == -1 {
			continue
		}
		userList = append(userList, &entity.GameUser{
			Uid:        v.UserInfo.Uid,
			Nickname:   v.UserInfo.Nickname,
			Avatar:     v.UserInfo.Avatar,
			Score:      int64(v.WinScore),
			SpreaderID: v.UserInfo.SpreaderID,
		})
	}
	for key, v := range r.clearUserArr {
		var temp *entity.GameUser
		for _, vv := range userList {
			if vv.Uid == key {
				temp = vv
				break
			}
		}
		if temp != nil {
			temp.Score = v.Score
		} else {
			userList = append(userList, v)
		}
	}
	var detail string
	if r.GameFrame.GetGameBureauData() != nil {
		bytes, _ := json.Marshal(r.GameFrame.GetGameBureauData())
		detail = string(bytes)
	} else {
		bytes, _ := json.Marshal([]any{})
		detail = string(bytes)
	}
	savaData := &entity.UserGameRecord{
		RoomID:     r.Id,
		GameType:   int(r.GameRule.GameType),
		Detail:     detail,
		UserList:   userList,
		CreateTime: time.Now().UnixMilli(),
	}
	if gameVideoRecord != nil {
		savaData.VideoRecordID = gameVideoRecord.Id.Hex()
	}
	if r.RoomCreator.CreatorType == enums.UnionCreatorType {
		savaData.UnionID = r.RoomCreator.UnionID
	} else {
		savaData.CreatorUid = r.RoomCreator.Uid
	}
	r.UserService.SaveUserGameRecord(savaData)
}

func (r *Room) allPlayedUsersLocked() map[string]*proto.RoomUser {
	result := make(map[string]*proto.RoomUser)
	for key, user := range r.users {
		if user != nil && utils.IndexOf(r.alreadyCostUserUidArr, key) != -1 {
			result[key] = user
		}
	}
	for key, clearUser := range r.clearUserArr {
		if clearUser == nil || result[key] != nil || utils.IndexOf(r.alreadyCostUserUidArr, key) == -1 {
			continue
		}
		result[key] = &proto.RoomUser{
			Uid:      key,
			WinScore: int(clearUser.Score),
			UserInfo: &proto.UserInfo{
				Uid:        key,
				Nickname:   clearUser.Nickname,
				SpreaderID: clearUser.SpreaderID,
				Avatar:     clearUser.Avatar,
			},
		}
	}
	return result
}
