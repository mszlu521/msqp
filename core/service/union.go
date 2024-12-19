package service

import (
	"common/logs"
	"context"
	"core/dao"
	"core/models/entity"
	"core/repo"
	"go.mongodb.org/mongo-driver/bson"
)

type UnionService struct {
	unionDao *dao.UnionDao
}

func (s *UnionService) FindUnionById(id int64) *entity.Union {
	union, err := s.unionDao.FindUnionByUnionID(context.Background(), id)
	if err != nil {
		logs.Info("find union by union id error: %v", err)
		return nil
	}
	return union
}

func (s *UnionService) FindUnionAndUpdate(ctx context.Context, matchData bson.M, saveData bson.M) (*entity.Union, error) {
	return s.unionDao.FindAndUpdate(ctx, matchData, saveData)
}

func NewUnionService(r *repo.Manager) *UnionService {
	return &UnionService{
		unionDao: dao.NewUnionDao(r),
	}
}
