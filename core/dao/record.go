package dao

import (
	"common/logs"
	"context"
	"core/models/entity"
	"core/repo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
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

func (d *RecordDao) CreateUserScoreChangeRecord(ctx context.Context, record *entity.UserScoreChangeRecord) error {
	collection := d.repo.Mongo.Db.Collection("userScoreChangeRecord")
	_, err := collection.InsertOne(ctx, record)
	return err
}

func (d *RecordDao) FindSafeBoxOperationRecordPage(ctx context.Context, startIndex int, count int, sortData bson.M, matchData bson.M) ([]*entity.SafeBoxRecord, int64, error) {
	collection := d.repo.Mongo.Db.Collection("safeBoxRecord")
	cursor, err := collection.Find(ctx,
		matchData,
		options.Find().SetSort(sortData).SetSkip(int64(startIndex)).SetLimit(int64(count)))
	if err != nil {
		logs.Error("FindSafeBoxOperationRecordPage err:%v", err)
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var list []*entity.SafeBoxRecord
	err = cursor.All(ctx, &list)
	return list, 0, err
}

func (d *RecordDao) CreateSafeBoxOperationRecord(ctx context.Context, record *entity.SafeBoxRecord) error {
	collection := d.repo.Mongo.Db.Collection("safeBoxRecord")
	_, err := collection.InsertOne(ctx, record)
	return err
}

func (d *RecordDao) SaveScoreModifyRecord(ctx context.Context, record *entity.ScoreModifyRecord) error {
	collection := d.repo.Mongo.Db.Collection("scoreModifyRecord")
	_, err := collection.InsertOne(ctx, record)
	return err
}

func NewRecordDao(m *repo.Manager) *RecordDao {
	return &RecordDao{
		repo: m,
	}
}
