package entity

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGameVideoRecordBsonRoundTripPreservesID(t *testing.T) {
	id := primitive.NewObjectID()
	want := GameVideoRecord{Id: id, RoomID: "room-1", GmeType: 6, Detail: "[]", CreateTime: 123}

	encoded, err := bson.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got GameVideoRecord
	if err := bson.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if got.Id != want.Id || got.RoomID != want.RoomID || got.GmeType != want.GmeType {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}
