package handler

import (
	"encoding/json"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"hall/models/response"
)

func TestUnionInfoResponseUsesSingleUpdateUserDataLayer(t *testing.T) {
	payload := &response.CreateUnionResp{
		Code:           0,
		UpdateUserData: unionInfoUpdateUserData([]string{"union"}),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	update, ok := decoded["updateUserData"].(map[string]any)
	if !ok {
		t.Fatalf("updateUserData = %#v, want object", decoded["updateUserData"])
	}
	if _, nested := update["updateUserData"]; nested {
		t.Fatalf("response contains nested updateUserData: %s", encoded)
	}
	if _, ok := update["unionInfo"]; !ok {
		t.Fatalf("response does not contain unionInfo: %s", encoded)
	}
}

func TestVideoRecordLookupUsesMongoObjectID(t *testing.T) {
	id := primitive.NewObjectID()
	match := bson.M{"_id": id}
	if got, ok := match["_id"].(primitive.ObjectID); !ok || got != id {
		t.Fatalf("video lookup filter = %#v, want _id ObjectID %s", match, id.Hex())
	}
}

func TestVideoRecordLookupRejectsInvalidObjectID(t *testing.T) {
	if _, err := primitive.ObjectIDFromHex("not-an-object-id"); err == nil {
		t.Fatal("invalid video record ID must be rejected")
	}
}

func TestMergeBsonMatchEnforcesRequiredFields(t *testing.T) {
	source := bson.M{"uid": "u1", "unionInfo.unionID": int64(999)}
	merged := mergeBsonMatch(source, bson.M{"unionInfo.unionID": int64(42)})
	if merged["unionInfo.unionID"] != int64(42) {
		t.Fatalf("required union filter was not enforced: %#v", merged)
	}
	if source["unionInfo.unionID"] != int64(999) {
		t.Fatalf("source match data was mutated: %#v", source)
	}
}

func TestMergeBsonMatchHandlesNilSource(t *testing.T) {
	merged := mergeBsonMatch(nil, bson.M{"unionID": int64(42)})
	if len(merged) != 1 || merged["unionID"] != int64(42) {
		t.Fatalf("nil source merge = %#v", merged)
	}
}

func TestParseInviteIDRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", " ", "abc", "0", "-1", "9223372036854775808"} {
		if id, ok := parseInviteID(value); ok || id != 0 {
			t.Fatalf("parseInviteID(%q) = %d, %v, want invalid", value, id, ok)
		}
	}
}

func TestParseInviteIDTrimsValidValue(t *testing.T) {
	id, ok := parseInviteID(" 12345 ")
	if !ok || id != 12345 {
		t.Fatalf("parseInviteID(valid) = %d, %v, want 12345", id, ok)
	}
}
