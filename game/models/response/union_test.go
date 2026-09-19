package response

import (
	"common"
	"encoding/json"
	"testing"
)

func TestHongBaoResponseUsesLegacyTopLevelShape(t *testing.T) {
	payload := HongBaoResp{
		Result: common.Result{
			Code: 0,
			Msg:  map[string]any{"score": 10},
		},
		UpdateUserData: map[string]any{
			"unionInfo": []any{map[string]any{"unionID": 42, "score": 10}},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	msg, ok := decoded["msg"].(map[string]any)
	if !ok || msg["score"] != float64(10) {
		t.Fatalf("unexpected hong bao payload: %s", encoded)
	}
	update, ok := decoded["updateUserData"].(map[string]any)
	if !ok {
		t.Fatalf("updateUserData = %#v, want object", decoded["updateUserData"])
	}
	if _, nested := msg["updateUserData"]; nested {
		t.Fatalf("hong bao response contains nested updateUserData: %s", encoded)
	}
	if _, ok := update["unionInfo"].([]any); !ok {
		t.Fatalf("unexpected updateUserData payload: %s", encoded)
	}
}
