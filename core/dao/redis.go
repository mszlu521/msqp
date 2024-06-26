package dao

import (
	"context"
	"core/repo"
	"fmt"
	"time"
)

const Prefix = "MSQP"
const AccountIdRedisKey = "AccountId"
const AccountIdBegin = 10000
const Register = "msqp_register_"

type RedisDao struct {
	repo *repo.Manager
}

func (d *RedisDao) Store(ctx context.Context, key string, value string) error {
	var err error
	if d.repo.Redis.Cli != nil {
		_, err = d.repo.Redis.Cli.Set(ctx, key, value, 0).Result()
	} else {
		_, err = d.repo.Redis.ClusterCli.Set(ctx, key, value, 0).Result()
	}
	return err
}
func (d *RedisDao) Get(ctx context.Context, key string) (string, error) {
	var err error
	var value string
	if d.repo.Redis.Cli != nil {
		value, err = d.repo.Redis.Cli.Get(ctx, key).Result()
	} else {
		value, err = d.repo.Redis.ClusterCli.Get(ctx, key).Result()
	}
	return value, err
}
func (d *RedisDao) NextAccountId() (string, error) {
	//自增 给一个前缀
	return d.incr(Prefix + ":" + AccountIdRedisKey)
}

func (d *RedisDao) incr(key string) (string, error) {
	//判断此key是否存在 不存在 set 存在就自增
	todo := context.TODO()
	var exist int64
	var err error
	//0 代表不存在
	if d.repo.Redis.Cli != nil {
		exist, err = d.repo.Redis.Cli.Exists(todo, key).Result()
	} else {
		exist, err = d.repo.Redis.ClusterCli.Exists(todo, key).Result()
	}
	if exist == 0 {
		//不存在
		if d.repo.Redis.Cli != nil {
			err = d.repo.Redis.Cli.Set(todo, key, AccountIdBegin, 0).Err()
		} else {
			err = d.repo.Redis.ClusterCli.Set(todo, key, AccountIdBegin, 0).Err()
		}
		if err != nil {
			return "", err
		}
	}
	var id int64
	if d.repo.Redis.Cli != nil {
		id, err = d.repo.Redis.Cli.Incr(todo, key).Result()
	} else {
		id, err = d.repo.Redis.ClusterCli.Incr(todo, key).Result()
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

func (d *RedisDao) CheckSmsCode(account string, code string) bool {
	v := d.repo.Redis.Cli.Get(context.TODO(), Register+account).Val()
	if v != code {
		return false
	}
	return true
}

func (d *RedisDao) Register(number string, code string, second time.Duration) error {
	return d.repo.Redis.Cli.Set(context.TODO(), Register+number, code, second).Err()
}

func NewRedisDao(m *repo.Manager) *RedisDao {
	return &RedisDao{
		repo: m,
	}
}
