package dao

import (
	"common/logs"
	"context"
	"core/models/entity"
	"core/repo"
)

type RecordDao struct {
	repo *repo.Manager
}

func (d *RecordDao) CreateUserScoreChangeRecordList(ctx context.Context, arr []*entity.UserScoreChangeRecord) error {
	if len(arr) == 0 {
		return nil
	}
	collection := d.repo.Mongo.Db.Collection("userScoreChangeRecord")
	// 转换为 interface{} 类型的切片
	documents := make([]interface{}, len(arr))
	for i, record := range arr {
		documents[i] = record
	}

	// 执行批量插入
	result, err := collection.InsertMany(ctx, documents)
	if err != nil {
		return err
	}
	logs.Info("Successfully inserted %d documents", len(result.InsertedIDs))
	return nil
}

func (d *RecordDao) CreateUserRebateRecord(ctx context.Context, data *entity.UserRebateRecord) error {
	collection := d.repo.Mongo.Db.Collection("userRebateRecord")
	_, err := collection.InsertOne(ctx, data)
	return err
}

func NewRecordDao(m *repo.Manager) *RecordDao {
	return &RecordDao{
		repo: m,
	}
}
