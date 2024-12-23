package dao

import (
	"context"
	"core/repo"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

const Prefix = "MSQP"
const AccountIdRedisKey = "AccountId"
const AccountIdBegin = 100000
const Register = "msqp_register_"
const UnionIdRedisKey = "UnionId"
const UnionIdBegin = 10000000
const InviteIdRedisKey = "InviteId"
const InviteIdBegin = 10000000

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
	if errors.Is(err, redis.Nil) {
		return "", nil
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

func (d *RedisDao) Delete(ctx context.Context, key string) error {
	return d.repo.Redis.Cli.Del(ctx, key).Err()
}

func (d *RedisDao) incrId(key string, begin int64) (int64, error) {
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
			err = d.repo.Redis.Cli.Set(todo, key, begin, 0).Err()
		} else {
			err = d.repo.Redis.ClusterCli.Set(todo, key, begin, 0).Err()
		}
		if err != nil {
			return -1, err
		}
	}
	var id int64
	if d.repo.Redis.Cli != nil {
		id, err = d.repo.Redis.Cli.Incr(todo, key).Result()
	} else {
		id, err = d.repo.Redis.ClusterCli.Incr(todo, key).Result()
	}
	if err != nil {
		return -1, err
	}
	return id, nil
}

func (d *RedisDao) NextUnionId() (int64, error) {
	id, err := d.incrId(Prefix+":"+UnionIdRedisKey, UnionIdBegin)
	if err != nil {
		return -1, err
	}
	return id, nil
}
func (d *RedisDao) NextInviteId() (int64, error) {
	id, err := d.incrId(Prefix+":"+InviteIdRedisKey, InviteIdBegin)
	if err != nil {
		return -1, err
	}
	return id, nil
}
func NewRedisDao(m *repo.Manager) *RedisDao {
	return &RedisDao{
		repo: m,
	}
}
