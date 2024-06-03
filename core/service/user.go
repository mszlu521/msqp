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
	"fmt"
	"framework/game"
	"framework/msError"
	hall "hall/models/request"
	"time"
)

type UserService struct {
	userDao    *dao.UserDao
	accountDao *dao.AccountDao
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
		user = &entity.User{}
		user.Uid = uid
		user.Gold = int64(game.Conf.GameConfig["startGold"]["value"].(float64))
		user.Avatar = utils.Default(info.Avatar, "Common/head_icon_default")
		user.Nickname = utils.Default(info.Nickname, fmt.Sprintf("%s%s", "码神", uid))
		user.Sex = info.Sex //0 男 1 女
		user.CreateTime = time.Now().UnixMilli()
		user.LastLoginTime = time.Now().UnixMilli()
		err = s.userDao.Insert(context.TODO(), user)
		if err != nil {
			logs.Error("[UserService] FindAndSaveUserByUid insert user err:%v", err)
			return nil, err
		}
	}
	return user, nil
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

func (s *UserService) GetUserData(phone string, uid string) (*entity.User, *msError.Error) {
	if uid != "" {
		//根据uid查询
		user, err := s.userDao.FindUserByUid(context.TODO(), fmt.Sprintf("%d", uid))
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

func NewUserService(r *repo.Manager) *UserService {
	return &UserService{
		userDao:    dao.NewUserDao(r),
		accountDao: dao.NewAccountDao(r),
	}
}
