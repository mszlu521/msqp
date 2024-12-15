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
	"sort"
	"strconv"
	"sync"
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
	Id                    string
	unionID               int64
	gameRule              proto.GameRule
	users                 map[string]*proto.RoomUser
	RoomCreator           *proto.RoomCreator
	GameFrame             GameFrame
	kickSchedules         map[string]*time.Timer
	startSchedulerID      *tasks.Task
	answerExitSchedule    *tasks.Task
	stopAnswerSchedules   chan struct{}
	union                 base.UnionBase
	roomDismissed         bool //房间是否被解散
	gameStarted           bool //房间是否开始
	askDismiss            []any
	chairCount            int
	currentUserCount      int
	roomType              TypeRoom
	createTime            time.Time
	lastNativeTime        time.Time
	hasFinishedOneBureau  bool
	hasStartedOneBureau   bool
	alreadyCostUserUidArr []string
	userJoinGameBureau    []string //记录玩家从第几局加入游戏
	maxBureau             int      //最大局数
	curBureau             int      //当前局数
	clearUserArr          map[string]*proto.UserRoomData
	UserService           *service.UserService
	RedisService          *service.RedisService
	dismissTick           int
	stopStartSchedulerID  chan struct{}
}

func (r *Room) GetMaxBureau() int {
	return r.maxBureau
}

func (r *Room) SetCurBureau(curBureau int) {
	r.curBureau = curBureau
}

func (r *Room) GetCurBureau() int {
	return r.curBureau
}

// IsDismissing 正在解散中
func (r *Room) IsDismissing() bool {
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
		if r.gameRule.GameType != enums.DGN {
			r.dismissRoom(session, enums.BureauFinished)
		}
	} else {
		// 移除不满足条件的玩家
		r.clearNonSatisfiedConditionsUser(session)
		// 通知更新所有玩家信息
		r.notifyUpdateAllUserInfo(session)
	}
}
func (r *Room) resetRoom(session *remote.Session) error {
	r.createTime = time.Now()
	r.lastNativeTime = time.Now()
	r.roomDismissed = false
	r.gameStarted = false
	r.hasFinishedOneBureau = false
	r.hasStartedOneBureau = false
	r.maxBureau = utils.Default(r.gameRule.Bureau, 8)
	r.curBureau = 0
	for _, v := range r.users {
		v.WinScore = 0
	}
	r.clearUserArr = make(map[string]*proto.UserRoomData)
	var err error
	r.GameFrame, err = NewGameFrame(r.gameRule, r, nil)
	if err != nil {
		return err
	}
	for _, v := range r.users {
		go r.addKickScheduleEvent(session, v)
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
	if r.roomDismissed {
		return biz.NotInRoom
	}
	user, ok := r.users[data.Uid]
	//检查是否允许新用户进入
	if !ok && !r.canEnter() {
		return biz.RoomPlayerCountFull
	}
	curUid := session.GetUid()
	_, ok1 := r.kickSchedules[curUid]
	if ok1 {
		r.kickSchedules[curUid].Stop()
		delete(r.kickSchedules, curUid)
	}
	//最多6人参加 0-5有6个号
	chairID := r.getEmptyChairID(data.Uid, false)
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
			UserInfo:   userInfo,
			ChairID:    chairID,
			UserStatus: enums.UserStatusNone,
		}
		r.users[data.Uid] = user
		r.currentUserCount++
		//2. 将房间号 推送给客户端 更新数据库 当前房间号存储起来
		err := r.UpdateUserInfoRoomPush(session, data.Uid)
		if err != nil {
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
		return biz.SqlError
	}
	//向其他玩家推送进入房间的消息
	roomUserInfo := &proto.RoomUser{
		UserInfo:   user.UserInfo,
		ChairID:    user.ChairID,
		UserStatus: user.UserStatus,
	}
	r.sendDataExceptUid(proto.OtherUserEntryRoomPushData(roomUserInfo), user.UserInfo.Uid, session.GetMsg())
	// 推送玩家自己进入房间的消息
	r.SelfEntryRoomPush(session, data.Uid)
	r.GameFrame.OnEventUserEntry(user, session)
	go r.addKickScheduleEvent(session, roomUserInfo)
	return nil
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
		"gameType":   r.gameRule.GameType,
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
	for _, v := range r.users {
		userInfoArr = append(userInfoArr, v)
	}
	data := map[string]any{
		"type":       proto.GetRoomSceneInfoPush,
		"pushRouter": "RoomMessagePush",
		"data": map[string]any{
			"roomID":          r.Id,
			"roomCreatorInfo": r.RoomCreator,
			"gameRule":        r.gameRule,
			"roomUserInfoArr": userInfoArr,
			"gameData":        r.GameFrame.GetEnterGameData(session),
		},
	}
	r.sendDataOne(data, session.GetUid(), session.GetMsg())
	if r.askDismiss != nil && len(r.askDismiss) > 0 {
		r.askForDismiss(session, session.GetUid(), nil)
	}
}

func (r *Room) addKickScheduleEvent(session *remote.Session, roomUser *proto.RoomUser) {
	r.Lock()
	defer r.Unlock()
	t, ok := r.kickSchedules[roomUser.UserInfo.Uid]
	if ok {
		t.Stop()
		delete(r.kickSchedules, roomUser.UserInfo.Uid)
	}
	r.kickSchedules[roomUser.UserInfo.Uid] = time.AfterFunc(30*time.Second, func() {
		logs.Info("kick 定时执行，代表 用户长时间未准备,uid=%v", roomUser.UserInfo.Uid)
		//取消定时任务
		timer, ok1 := r.kickSchedules[roomUser.UserInfo.Uid]
		if ok1 {
			timer.Stop()
			delete(r.kickSchedules, roomUser.UserInfo.Uid)
		}
		//需要判断用户是否该踢出
		user, ok2 := r.users[roomUser.UserInfo.Uid]
		if ok2 {
			if user.UserStatus < enums.Ready {
				r.kickUser(user, session)
				//踢出房间之后，需要判断是否可以解散房间
				if len(r.users) == 0 {
					r.dismissRoom(session, 0)
				}
			}
		}
	})
}

func (r *Room) kickUser(user *proto.RoomUser, session *remote.Session) {
	r.Lock()
	defer r.Unlock()
	if r.gameStarted {
		r.GameFrame.OnEventUserOffLine(user, session)
	}
	r.userLeaveRoomNotify([]*proto.RoomUser{user}, session)
	//通知其他人用户离开房间
	r.sendData(proto.OtherUserEntryRoomPushData(user), session.GetMsg())
	delete(r.users, user.UserInfo.Uid)
	r.currentUserCount--
	//关于此用户的定时器停止
}

func (r *Room) dismissRoom(session *remote.Session, reason enums.RoomDismissReason) {
	if r.TryLock() {
		r.Lock()
		defer r.Unlock()
	}
	if r.roomDismissed {
		return
	}
	r.roomDismissed = true
	//将redis中房间信息删除掉
	r.RedisService.Delete(r.Id)
	//解散 将union当中存储的room信息 删除掉
	r.cancelAllScheduler()
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
		r.union.DismissRoom(r.Id)
		r.destroyRoom(reason, session)
		r.sendData(proto.RoomDismissPushData(reason), session.GetMsg())
	} else {
		// 清除掉线玩家
		r.clearOfflineUser(session)
		if r.currentUserCount == 0 {
			r.union.DismissRoom(r.Id)
			r.destroyRoom(reason, session)
			return
		}
		r.notifyUpdateAllUserInfo(session)
		r.destroyRoom(reason, session)
		r.sendData(proto.RoomDismissPushData(reason), session.GetMsg())
		r.resetRoom(session)
	}
}

func (r *Room) cancelAllScheduler() {
	if r.startSchedulerID != nil {
		r.startSchedulerID.Stop()
		r.startSchedulerID = nil
	}
	//需要将房间所有的任务 都取消掉
	for uid, v := range r.kickSchedules {
		v.Stop() //阻塞
		delete(r.kickSchedules, uid)
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
	if r.hasStartedOneBureau && user.UserInfo.Score < r.gameRule.ScoreLowLimit {
		r.sendPopDialogContent(biz.LeaveRoomGoldExceedLimit, []string{user.UserInfo.Uid}, session)
		r.kickUser(user, session)
		return
	}
	if user.UserStatus&enums.Ready == 0 {
		user.UserStatus |= enums.Ready
		user.UserStatus |= enums.Dismiss
	} else {
		logs.Info("用户已经准备过了")
		return
	}
	r.sendData(proto.UserReadyPushData(user.ChairID), session.GetMsg())
	if r.efficacyStartRoom() {
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
		if r.isShouldSchedulerStart() {
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
	users := make([]string, 0)
	for _, v := range r.users {
		users = append(users, v.UserInfo.Uid)
	}
	return users
}

func (r *Room) getEmptyChairID(uid string, isWatch bool) int {
	r.Lock()
	defer r.Unlock()
	user, ok := r.users[uid]
	if ok {
		return user.ChairID
	}
	isWatch = isWatch || r.hasStartedOneBureau
	used := make(map[int]struct{})
	for _, v := range r.users {
		used[v.ChairID] = struct{}{}
	}
	chairID := 0
	if isWatch {
		chairID = r.chairCount
	}
	_, exist := used[chairID]
	if exist {
		chairID++
	}
	return chairID
}

func (r *Room) IsStartGame() bool {
	//房间内准备的人数 已经大于等于 最小开始游戏人数
	userReadyCount := 0
	for _, v := range r.users {
		if v.UserStatus == enums.Ready {
			userReadyCount++
		}
	}
	if r.gameRule.GameType == enums.ZNMJ {
		if len(r.users) == userReadyCount && userReadyCount >= r.gameRule.MaxPlayerCount {
			return true
		}
	}
	if len(r.users) == userReadyCount && userReadyCount >= r.gameRule.MinPlayerCount {
		return true
	}
	return false
}

func (r *Room) startGame(session *remote.Session, user *proto.RoomUser) {
	if r.gameStarted {
		return
	}
	if r.startSchedulerID != nil {
		r.startSchedulerID.Stop()
		r.startSchedulerID = nil
	}
	for k, v := range r.kickSchedules {
		v.Stop()
		delete(r.kickSchedules, k)
	}
	if r.maxBureau > 0 {
		//第一局游戏开局时收取房费
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

func NewRoom(uid string, roomId string, unionID int64, rule proto.GameRule, u base.UnionBase, session *remote.Session) (*Room, error) {
	r := &Room{
		Id:                   roomId,
		unionID:              unionID,
		gameRule:             rule,
		users:                make(map[string]*proto.RoomUser),
		kickSchedules:        make(map[string]*time.Timer),
		union:                u,
		roomType:             TypeRoomNone,
		chairCount:           rule.MaxPlayerCount,
		stopAnswerSchedules:  make(chan struct{}, 1),
		stopStartSchedulerID: make(chan struct{}, 1),
	}
	if unionID == 1 {
		//普通房间
		r.RoomCreator = &proto.RoomCreator{
			CreatorType: enums.UserCreatorType,
			Uid:         uid,
			UnionID:     unionID,
		}
	} else {
		r.RoomCreator = &proto.RoomCreator{
			CreatorType: enums.UnionCreatorType,
			Uid:         uid,
			UnionID:     unionID,
		}
	}
	var err error
	r.GameFrame, err = NewGameFrame(rule, r, session)
	if err != nil {
		return nil, err
	}
	go r.stopSchedule()
	return r, nil
}

func (r *Room) GetUsers() map[string]*proto.RoomUser {
	return r.users
}
func (r *Room) GetId() string {
	return r.Id
}
func (r *Room) GameMessageHandle(session *remote.Session, msg []byte) {
	//需要游戏去处理具体的消息
	user, ok := r.users[session.GetUid()]
	if !ok {
		return
	}
	r.GameFrame.GameMessageHandle(user, session, msg)
}

func (r *Room) askForDismiss(session *remote.Session, uid string, exist any) {
	r.Lock()
	defer r.Unlock()
	askUser := r.users[uid]
	if askUser.UserStatus&enums.Dismiss == 0 || askUser.ChairID >= r.chairCount {
		r.userLeaveRoomRequest(session)
		return
	}
	//所有同意座次的数组
	if (r.askDismiss == nil || len(r.askDismiss) == 0) && (exist != nil && exist.(bool)) {
		//同意解散
		if r.askDismiss == nil {
			r.askDismiss = make([]any, r.chairCount)
		}
		for i := 0; i < r.chairCount; i++ {
			r.askDismiss[i] = nil
		}
		r.dismissTick = proto.ExitWaitSecond
		r.answerExitSchedule = tasks.NewTask("answerExitSchedule", 1*time.Second, func() {
			r.dismissTick--
			if r.dismissTick == 0 {
				r.stopAnswerSchedules <- struct{}{}
				for _, v := range r.users {
					if v.UserStatus&enums.Dismiss > 0 && v.ChairID < r.chairCount {
						r.askForDismiss(session, v.UserInfo.Uid, true)
					}
				}
			}
		})
	}
	if r.askDismiss == nil {
		return
	}
	if r.askDismiss[askUser.ChairID] != nil {
		return
	}
	r.askDismiss[askUser.ChairID] = exist

	nameArr := make([]string, r.chairCount)
	avatarArr := make([]string, r.chairCount)
	onlineArr := make([]bool, len(r.users))
	for _, v := range r.users {
		if v.UserStatus&enums.Dismiss > 0 && v.ChairID < r.chairCount {
			nameArr[v.ChairID] = v.UserInfo.Nickname
			avatarArr[v.ChairID] = v.UserInfo.Avatar
			onlineArr[v.ChairID] = v.UserStatus&enums.Offline == 0
		}
	}
	for _, v := range r.users {
		if v.UserStatus&enums.Dismiss > 0 && v.ChairID < r.chairCount {
			data := proto.DismissPushData{
				NameArr:    nameArr,
				ChairIDArr: r.askDismiss,
				AskChairId: askUser.ChairID,
				OnlineArr:  onlineArr,
				AvatarArr:  avatarArr,
				Tm:         r.dismissTick,
			}
			r.sendDataOne(proto.AskForDismissPushData(&data), v.UserInfo.Uid, session.GetMsg())
		}
	}
	//不同意直接取消解散申请
	if exist != nil && !exist.(bool) {
		if r.answerExitSchedule != nil {
			r.answerExitSchedule.Stop()
			r.answerExitSchedule = nil
		}
		r.askDismiss = nil
	} else if exist != nil && exist.(bool) {
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
			if r.answerExitSchedule != nil {
				r.answerExitSchedule.Stop()
				r.answerExitSchedule = nil
			}
			r.dismissRoom(session, enums.UserDismiss)
		}
	}
}

func (r *Room) sendData(data any, msg *stream.Msg) {
	r.SendData(msg, r.AllUsers(), data)
}
func (r *Room) sendDataOne(data any, uid string, msg *stream.Msg) {
	r.SendData(msg, []string{uid}, data)
}
func (r *Room) sendDataMany(data any, uids []string, msg *stream.Msg) {
	r.SendData(msg, uids, data)
}
func (r *Room) userLeaveRoomRequest(session *remote.Session) {
	user, ok := r.users[session.GetUid()]
	if ok {
		if r.gameStarted &&
			(user.UserStatus&enums.Playing != 0 && !r.GameFrame.IsUserEnableLeave(user.ChairID)) {
			r.sendPopDialogContent(biz.CanNotLeaveRoom, []string{user.UserInfo.Uid}, session)
			r.sendData(proto.UserLeaveRoomResponsePushData(user.ChairID), session.GetMsg())
		} else {
			r.userLeaveRoom(session)
		}
	}
}

// userChangeSeat 玩家换座位
func (r *Room) userChangeSeat(session *remote.Session, fromChairID int, toChairID int) {
	if fromChairID < 0 || toChairID < 0 {
		return
	}
	user, ok := r.users[session.GetUid()]
	if !ok {
		return
	}
	if user.UserStatus == enums.Playing {
		//正在游戏不能换座位
		return
	}
	if !r.gameStarted && user.UserStatus == enums.Ready {
		//如果游戏未开始，且玩家已准备，则重置用户状态
		user.UserStatus = enums.UserStatusNone
	}
	//目标位置有人 不能换座位
	if r.getUserByChairID(toChairID) != nil {
		return
	}
	//判断用户是否有足够的积分
	if toChairID < r.chairCount && user.UserInfo.Score < r.gameRule.ScoreLowLimit {
		return
	}
	user.ChairID = toChairID
	//推送给所有用户
	r.sendData(proto.GetUserChangeSeatPush(fromChairID, toChairID, user.UserInfo.Uid), session.GetMsg())
}

func (r *Room) userChat(session *remote.Session, data request.RoomMessageData) {
	user, ok := r.users[session.GetUid()]
	if !ok {
		return
	}
	fromChairID := user.ChairID
	r.SendDataAll(session.GetMsg(), proto.UserChatPushData(fromChairID, data.ToChairID, data.Msg))

}

func (r *Room) userLeaveRoom(session *remote.Session) {
	user, ok := r.users[session.GetUid()]
	if !ok {
		log.Printf("leave room fail,roomId:%s, not this user: %v\n", r.Id, user.UserInfo)
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
		r.dismissRoom(session, enums.DismissNone)
	}
}

func (r *Room) efficacyDismissRoom() bool {
	if r.roomDismissed {
		return false
	}
	return r.currentUserCount == 0
}

func (r *Room) getUserByChairID(chairID int) *proto.RoomUser {
	for _, value := range r.users {
		if value.ChairID == chairID {
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
	return (userCount == readyCount && userCount >= r.gameRule.MinPlayerCount) || readyCount == r.gameRule.MaxPlayerCount
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
	return readyCount >= 4 && readyCount >= r.gameRule.MinPlayerCount
}

// 正在解散中
func (r *Room) isDismissing() bool {
	if r.askDismiss == nil {
		return false
	}
	return len(r.askDismiss) > 0
}

func (r *Room) canEnter() bool {
	hasEmpty := r.hasEmptyChair()
	canWatch := r.gameRule.CanWatch
	canEnter := r.gameRule.CanEnter && (r.gameRule.GameType != enums.PDK)
	if r.hasStartedOneBureau {
		return canEnter && (hasEmpty || (canWatch && r.currentUserCount < 20))
	}
	return hasEmpty || (canWatch && r.currentUserCount < 20)
}

func (r *Room) hasEmptyChair() bool {
	seatCount := 0
	for _, v := range r.users {
		if v.ChairID == -1 {
			continue
		}
		seatCount++
	}
	return seatCount < r.chairCount
}

func (r *Room) checkEntryRoom(userInfo *proto.UserInfo) *msError.Error {
	if r.RoomCreator.CreatorType == enums.UserCreatorType {
		//普通房间
		if r.gameRule.PayType == proto.MyPay {
			//检查钻石是否足够
			if userInfo.Uid == r.RoomCreator.Uid {
				if userInfo.Gold < int64(r.gameRule.PayDiamond) {
					return biz.NotEnoughGold
				}
			}
		} else {
			if userInfo.Gold < int64(r.gameRule.PayDiamond) {
				return biz.NotEnoughGold
			}
		}
	}
	return nil
}

func (r *Room) sendDataExceptUid(data any, uid string, msg *stream.Msg) {
	var uids []string
	for _, v := range r.users {
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

func (r *Room) clearOfflineUser(session *remote.Session) {
	for _, v := range r.users {
		if v.UserStatus&enums.Offline != 0 {
			r.kickUser(v, session)
		}
	}
}

func (r *Room) notifyUpdateAllUserInfo(session *remote.Session) {
	for _, v := range r.users {
		if v.ChairID > r.chairCount {
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
				CreateTime:       time.Now().Unix(),
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
	roomPayRule := r.gameRule.RoomPayRule
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
	totalRebateCount := 0
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
		scoreChangeRecordArr = append(scoreChangeRecordArr, &entity.UserScoreChangeRecord{
			CreateTime:       time.Now().Unix(),
			Uid:              newUserData.Uid,
			Nickname:         newUserData.Nickname,
			UnionID:          r.RoomCreator.UnionID,
			ChangeCount:      int64(-count),
			LeftCount:        int64(newUnionInfo.Score),
			LeftSafeBoxCount: int64(newUnionInfo.SafeScore),
			ChangeType:       enums.GameStartUnionChou,
			Describe:         fmt.Sprintf("抽取房费%d", count),
		})
		totalRebateCount += count - roomPayRule.EveryAgentFixedScore
		// 计算代理固定返利
		if roomPayRule.EveryAgentFixedScore > 0 {
			r.execRebate(
				r.RoomCreator.UnionID,
				r.Id,
				r.gameRule.GameType, v.UserInfo,
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
}
func (r *Room) execRebate(unionID int64, roomId string, gameType enums.GameType, userInfo *proto.UserInfo, lowPartnerUnionInfo *entity.UnionInfo, lowUid string, spreaderID string, count int, bigWin bool, isOneDraw bool, session *remote.Session) {
	if spreaderID == "" {
		return
	}
	lowPartnerRebateRate := 0
	if lowPartnerUnionInfo != nil {
		lowPartnerRebateRate = lowPartnerUnionInfo.RebateRate
	}
	userData, err := r.UserService.FindUserByUid(context.TODO(), spreaderID)
	if err != nil {
		logs.Error("FindUserByUid err : %v", err)
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
	getScore := count * (unionInfo.RebateRate - lowPartnerRebateRate)
	// 第二步：向下取整到两位小数
	getScore = int(math.Floor(float64(getScore*100)) / 100)
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
		r.updateUserDataNotify(map[string]any{"unionInfo": newUserData.UnionInfo}, session)
		// 记录下级玩家贡献的的返利数
		if lowUid != "" {
			matchData = bson.M{"unionInfo.unionID": unionID, "uid": lowUid}
			saveData = bson.M{"$inc": bson.M{"unionInfo.$.todayProvideRebate": getScore}}
			r.UserService.UpdateUserData(matchData, saveData)
		}
		// 添加记录
		createData := &entity.UserRebateRecord{
			CreateTime: time.Now().Unix(),
			Uid:        spreaderID,
			RoomID:     roomId,
			GameType:   int(gameType),
			UnionID:    unionID,
			PlayerUid:  userInfo.Uid,
			TotalCount: count,
			GainCount:  getScore,
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
	if r.gameRule.RoomPayRule.RebateType != enums.One {
		return
	}
	dataList := make(map[string]*proto.UserRoomData)
	for _, v := range dataArr {
		dataList[v.Uid] = &proto.UserRoomData{
			Uid:      v.Uid,
			WinScore: v.Score,
		}
	}
	rebateList := r.calculateRebate(dataList)
	avgRebateCount := 0
	if !r.gameRule.RoomPayRule.IsAvg {
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
		if !r.gameRule.RoomPayRule.IsAvg {
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
		r.updateUserDataNotify(map[string]any{"unionInfo": newUserData.UnionInfo}, session)
		r.updateRoomUserInfo(proto.BuildGameRoomUserInfoWithUnion(newUserData, r.RoomCreator.UnionID, session.GetMsg().ConnectorId), false, session)
		r.execRebate(r.RoomCreator.UnionID, r.Id, r.gameRule.GameType, user.UserInfo, nil, "", uid, rebateCount, false, true, session)
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
		if user.UserInfo.Score < r.gameRule.ScoreDismissLimit {
			if r.gameRule.GameType == enums.PDK || r.gameRule.GameType == enums.ZNMJ {
				r.dismissRoom(session, enums.UserDismiss)
			} else {
				if r.gameRule.CanEnter && r.gameRule.CanWatch {
					r.userChangeSeat(session, user.ChairID, r.getEmptyChairID("", true))
				} else {
					kickUidArr = append(kickUidArr, user.UserInfo.Uid)
					kickChairIDArr = append(kickChairIDArr, user.ChairID)
				}
				_, ok := r.clearUserArr[user.UserInfo.Uid]
				if ok {
					r.clearUserArr[user.UserInfo.Uid].WinScore += user.WinScore
				} else {
					r.clearUserArr[user.UserInfo.Uid] = &proto.UserRoomData{
						Uid:        user.UserInfo.Uid,
						Score:      user.WinScore,
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
func (r *Room) calculateRebate(dataList map[string]*proto.UserRoomData) map[string]int {
	roomPayRule := r.gameRule.RoomPayRule
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

func (r *Room) getBinWinUidArr(dataList map[string]*proto.UserRoomData) []string {
	var userWinScoreArr []*proto.UserRoomData
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
	roomPayRule := r.gameRule.RoomPayRule
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
