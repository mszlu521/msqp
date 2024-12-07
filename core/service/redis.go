package service

import (
	"context"
	"core/dao"
	"core/repo"
)

type RedisService struct {
	redisDao *dao.RedisDao
}

func (s *RedisService) Store(key string, value string) error {
	return s.redisDao.Store(context.TODO(), key, value)
}

func (s *RedisService) Get(ctx context.Context, key string) (string, error) {
	return s.redisDao.Get(ctx, key)
}

func (s *RedisService) Delete(key string) error {
	return s.redisDao.Delete(context.TODO(), key)
}

func NewRedisService(r *repo.Manager) *RedisService {
	return &RedisService{
		redisDao: dao.NewRedisDao(r),
	}
}
