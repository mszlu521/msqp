package service

import (
	"common/biz"
	"common/logs"
	"common/utils"
	"connector/models/request"
	"context"
	"core/dao"
	"core/models/entity"
	"core/repo"
	"encoding/json"
	"fmt"
	"framework/game"
	"framework/msError"
	"framework/pusher"
	"framework/remote"
	"framework/stream"
	"go.mongodb.org/mongo-driver/bson"
	hall "hall/models/request"
	"math"
	"reflect"
	"time"
)

type UserService struct {
	userDao    *dao.UserDao
	accountDao *dao.AccountDao
	recordDao  *dao.RecordDao
}

func (s *UserService) FindAndSaveUserByUid(ctx context.Context, uid string, info request.UserInfo) (*entity.User, error) {
	//查询mongo 有 返回 没有 新增
	user, err := s.userDao.FindUserByUid(ctx, uid)
	if err != nil {
		logs.Error("[UserService] FindAndSaveUserByUid  user err:%v", err)
		return nil, err
	}
	if user == nil {
		//save
		startGold, err := configuredStartGold()
		if err != nil {
			logs.Error("[UserService] invalid startGold config: %v", err)
			return nil, err
		}
		user = &entity.User{}
		user.Uid = uid
		user.Gold = startGold
		user.Avatar = utils.Default(info.Avatar, "Common/head_icon_default")
		user.Nickname = utils.Default(info.Nickname, fmt.Sprintf("%s%s", "码神", uid))
		user.Sex = info.Sex //0 男 1 女
		user.CreateTime = time.Now().UnixMilli()
		user.LastLoginTime = time.Now().UnixMilli()
		user.UnionInfo = []*entity.UnionInfo{}
		user.InviteMsg = []*entity.InviteMsg{}
		err = s.userDao.Insert(context.TODO(), user)
		if err != nil {
			logs.Error("[UserService] FindAndSaveUserByUid insert user err:%v", err)
			return nil, err
		}
	}
	if user.UnionInfo == nil {
		user.UnionInfo = []*entity.UnionInfo{}
	}
	return user, nil
}

func configuredStartGold() (int64, error) {
	if game.Conf == nil {
		return 0, fmt.Errorf("game config is nil")
	}
	startGoldConfig, ok := game.Conf.GameConfig["startGold"]
	if !ok {
		return 0, fmt.Errorf("startGold config is missing")
	}
	value, ok := startGoldConfig["value"]
	if !ok || value == nil {
		return 0, fmt.Errorf("startGold.value is missing")
	}

	var number float64
	switch v := value.(type) {
	case float64:
		number = v
	case float32:
		number = float64(v)
	case int:
		number = float64(v)
	case int8:
		number = float64(v)
	case int16:
		number = float64(v)
	case int32:
		number = float64(v)
	case int64:
		number = float64(v)
	case uint:
		number = float64(v)
	case uint8:
		number = float64(v)
	case uint16:
		number = float64(v)
	case uint32:
		number = float64(v)
	case uint64:
		number = float64(v)
	case json.Number:
		parsed, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("startGold.value is not numeric: %w", err)
		}
		number = parsed
	default:
		return 0, fmt.Errorf("startGold.value has unsupported type %s", reflect.TypeOf(value))
	}
	if math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || math.Trunc(number) != number || number >= float64(uint64(1)<<63) {
		return 0, fmt.Errorf("startGold.value must be a non-negative integer")
	}
	return int64(number), nil
}

func (s *UserService) FindUserByUid(ctx context.Context, uid string) (*entity.User, *msError.Error) {
	//查询mongo 有 返回 没有 新增
	user, err := s.userDao.FindUserByUid(ctx, uid)
	if err != nil {
		logs.Error("[UserService] FindUserByUid  user err:%v", err)
		return nil, biz.SqlError
	}
	return user, nil
}

func (s *UserService) UpdateUserAddressByUid(uid string, req hall.UpdateUserAddressReq) error {
	user := &entity.User{
		Uid:      uid,
		Address:  req.Address,
		Location: req.Location,
	}
	err := s.userDao.UpdateUserAddressByUid(context.TODO(), user)
	if err != nil {
		logs.Error("userDao.UpdateUserAddressByUid err:%v", err)
		return err
	}
	return nil
}

func (s *UserService) BindPhone(uid string, phone string) *msError.Error {
	ac, err := s.accountDao.FindAccountByPhone(context.TODO(), phone)
	if err != nil {
		logs.Error("FindAccountByPhone err : %v", err)
		return biz.SqlError
	}
	if ac != nil {
		return biz.PhoneAlreadyBind
	}
	err = s.accountDao.UpdatePhone(context.TODO(), uid, phone)
	if err != nil {
		logs.Error("UpdatePhone err : %v", err)
		return biz.SqlError
	}
	err = s.userDao.UpdatePhone(context.TODO(), uid, phone)
	if err != nil {
		logs.Error("UpdatePhone err : %v", err)
		return biz.SqlError
	}
	return nil
}

func (s *UserService) UpdateUserRealName(uid string, info string) *msError.Error {
	err := s.userDao.UpdateUserRealName(context.TODO(), uid, info)
	if err != nil {
		logs.Error("UpdatePhone err : %v", err)
		return biz.SqlError
	}
	return nil
}

func (s *UserService) UpdateEmailArr(uid string, emailArr string) error {
	return s.userDao.UpdateEmailArr(context.TODO(), uid, emailArr)
}

func (s *UserService) UpdateEmailArrIfCurrent(uid string, current string, emailArr string) (bool, error) {
	return s.userDao.UpdateEmailArrIfCurrent(context.TODO(), uid, current, emailArr)
}

func (s *UserService) GetUserData(phone string, uid string) (*entity.User, *msError.Error) {
	if uid != "" {
		//根据uid查询
		user, err := s.userDao.FindUserByUid(context.TODO(), uid)
		if err != nil {
			logs.Error("FindUserByUid err : %v", err)
			return nil, biz.SqlError
		}
		return user, nil
	} else if phone != "" {
		//根据uid查询
		user, err := s.userDao.FindUserByPhone(context.TODO(), phone)
		if err != nil {
			logs.Error("FindUserByPhone err : %v", err)
			return nil, biz.SqlError
		}
		return user, nil
	}
	return nil, biz.RequestDataError
}

func (s *UserService) UpdateUserRoomId(ctx context.Context, uid string, roomId string) error {
	err := s.userDao.UpdateUserRoomId(ctx, uid, roomId)
	if err != nil {
		logs.Error("UpdateUserRoomId err : %v", err)
		return biz.SqlError
	}
	return nil
}

func (s *UserService) UpdateUserDataScoreInc(uid string, unionID int64, score int) *entity.User {
	ctx := context.Background()
	matchData := bson.M{"unionInfo.unionID": unionID, "uid": uid}
	saveData := bson.M{"$inc": bson.M{"unionInfo.$.score": score}}
	user, err := s.userDao.FindAndUpdate(ctx, matchData, saveData)
	if err != nil {
		logs.Error("FindAndUpdate err : %v", err)
		return nil
	}
	return user
}

func (s *UserService) UpdateUserData(matchData bson.M, saveData bson.M) *entity.User {
	ctx := context.Background()
	user, err := s.userDao.FindAndUpdate(ctx, matchData, saveData)
	if err != nil {
		logs.Error("FindAndUpdate err : %v", err)
		return nil
	}
	return user
}

func (s *UserService) SaveUserScoreChangeRecordList(arr []*entity.UserScoreChangeRecord) error {
	err := s.recordDao.CreateUserScoreChangeRecordList(context.TODO(), arr)
	if err != nil {
		logs.Error("saveUserScoreChangeRecordList err : %v", err)
		return biz.SqlError
	}
	return nil
}

func (s *UserService) SaveUserRebateRecord(data *entity.UserRebateRecord) error {
	return s.recordDao.CreateUserRebateRecord(context.TODO(), data)
}

func (s *UserService) SaveUserScoreChangeRecord(record *entity.UserScoreChangeRecord) error {
	return s.recordDao.CreateUserScoreChangeRecord(context.Background(), record)
}

func (s *UserService) UpdateUserDataNotify(uid string, frontendId string, data map[string]any, session *remote.Session) {
	if frontendId != "" {
		data["pushRouter"] = "UpdateUserInfoPush"
		msg := session.GetMsg()
		pusher.GetPusher().Push(msg, []stream.PushUser{
			{Uid: uid, ConnectorId: frontendId},
		}, data, "ServerMessagePush")
	}
}

func (s *UserService) SaveGameVideoRecord(data *entity.GameVideoRecord) {
	s.recordDao.SaveGameVideoRecord(context.Background(), data)
}

func (s *UserService) SaveUserGameRecord(data *entity.UserGameRecord) {
	s.recordDao.SaveUserGameRecord(context.Background(), data)
}
func NewUserService(r *repo.Manager) *UserService {
	return &UserService{
		userDao:    dao.NewUserDao(r),
		accountDao: dao.NewAccountDao(r),
		recordDao:  dao.NewRecordDao(r),
	}
}
