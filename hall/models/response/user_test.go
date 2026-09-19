package response

import (
	"common"
	"encoding/json"
	"testing"
)

func TestUpdateEmailResponseUsesLegacyUserDataShape(t *testing.T) {
	payload := UpdateEmailRes{
		Result:         common.Result{Code: 0},
		UpdateUserData: UpdateEmailData{EmailArr: `[{"id":"mail-1","isRead":true}]`},
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
	if !ok || update["emailArr"] != `[{"id":"mail-1","isRead":true}]` {
		t.Fatalf("unexpected updateUserData payload: %s", encoded)
	}
	if _, nested := update["updateUserData"]; nested {
		t.Fatalf("email response contains nested updateUserData: %s", encoded)
	}
}
