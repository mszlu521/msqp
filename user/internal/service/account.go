package service

import (
	"common/biz"
	"common/logs"
	"context"
	"core/dao"
	"core/models/entity"
	"core/models/requests"
	"core/repo"
	"framework/msError"
	"time"
	"user/pb"
)

//创建账号

type AccountService struct {
	accountDao *dao.AccountDao
	redisDao   *dao.RedisDao
	pb.UnimplementedUserServiceServer
}

func NewAccountService(manager *repo.Manager) *AccountService {
	return &AccountService{
		accountDao: dao.NewAccountDao(manager),
		redisDao:   dao.NewRedisDao(manager),
	}
}
func (a *AccountService) Login(ctx context.Context, req *pb.LoginParams) (*pb.LoginResponse, error) {
	if req.LoginPlatform == requests.MobilePhone {
		smsCode := req.Password
		if smsCode == "" {
			//验证码错误
			return &pb.LoginResponse{}, msError.GrpcError(biz.SmsCodeError)
		}
		if !a.redisDao.CheckSmsCode(req.Account, smsCode) {
			//验证码错误
			return &pb.LoginResponse{}, msError.GrpcError(biz.SmsCodeError)
		}
		//检查数据库是否有此用户 有直接登录
		ac, err := a.accountDao.FindAccountByAccount(context.TODO(), req.Account)
		if err != nil {
			logs.Error("FindAccountByAccount err:%v", err)
			return &pb.LoginResponse{}, msError.GrpcError(biz.SqlError)
		}
		if ac != nil {
			//直接返回id
			return &pb.LoginResponse{
				Uid: ac.Uid,
			}, nil
		}
		//注册
		ac, dbErr := a.phoneRegister(req.Account)
		if dbErr != nil {
			return &pb.LoginResponse{}, msError.GrpcError(dbErr)
		}
		return &pb.LoginResponse{
			Uid: ac.Uid,
		}, nil
	}
	return &pb.LoginResponse{}, msError.GrpcError(biz.RequestDataError)
}
func (a *AccountService) GetSMSCode(ctx context.Context, req *pb.GetSMSCodeParams) (*pb.Empty, error) {
	code := "123456"
	err := a.redisDao.Register(req.PhoneNumber, code, time.Minute*10)
	if err != nil {
		return &pb.Empty{}, msError.GrpcError(biz.SmsSendFailed)
	}
	return &pb.Empty{}, nil
}
func (a *AccountService) Register(ctx context.Context, req *pb.RegisterParams) (*pb.RegisterResponse, error) {
	//写注册的业务逻辑
	if req.LoginPlatform == requests.WeiXin {
		ac, err := a.wxRegister(req)
		if err != nil {
			return &pb.RegisterResponse{}, msError.GrpcError(err)
		}
		return &pb.RegisterResponse{
			Uid: ac.Uid,
		}, nil
	} else if req.LoginPlatform == requests.MobilePhone {
		smsCode := req.SmsCode
		if smsCode == "" {
			//验证码错误
			return &pb.RegisterResponse{}, msError.GrpcError(biz.SmsCodeError)
		}
		if !a.redisDao.CheckSmsCode(req.Account, smsCode) {
			//验证码错误
			return &pb.RegisterResponse{}, msError.GrpcError(biz.SmsCodeError)
		}
		//检查数据库是否有此用户 有直接登录
		ac, err := a.accountDao.FindAccountByAccount(context.TODO(), req.Account)
		if err != nil {
			logs.Error("FindAccountByAccount err:%v", err)
			return &pb.RegisterResponse{}, msError.GrpcError(biz.SqlError)
		}
		if ac != nil {
			//直接返回id
			return &pb.RegisterResponse{
				Uid: ac.Uid,
			}, nil
		}
		//注册
		ac, dbErr := a.phoneRegister(req.Account)
		if dbErr != nil {
			return &pb.RegisterResponse{}, msError.GrpcError(dbErr)
		}
		return &pb.RegisterResponse{
			Uid: ac.Uid,
		}, nil
	}
	return &pb.RegisterResponse{}, nil
}

func (a *AccountService) wxRegister(req *pb.RegisterParams) (*entity.Account, *msError.Error) {
	//1.封装一个account结构 将其存入数据库  mongo 分布式id objectID
	ac := &entity.Account{
		WxAccount:  req.Account,
		CreateTime: time.Now(),
	}
	//2.需要生成几个数字做为用户的唯一id  redis自增
	uid, err := a.redisDao.NextAccountId()
	if err != nil {
		return ac, biz.SqlError
	}
	ac.Uid = uid
	err = a.accountDao.SaveAccount(context.TODO(), ac)
	if err != nil {
		return ac, biz.SqlError
	}
	return ac, nil
}

func (a *AccountService) phoneRegister(account string) (*entity.Account, *msError.Error) {
	ac := &entity.Account{
		PhoneAccount: account,
		CreateTime:   time.Now(),
	}
	//2.需要生成几个数字做为用户的唯一id  redis自增
	uid, err := a.redisDao.NextAccountId()
	if err != nil {
		return ac, biz.SqlError
	}
	ac.Uid = uid
	err = a.accountDao.SaveAccount(context.TODO(), ac)
	if err != nil {
		return ac, biz.SqlError
	}
	return ac, nil
}
