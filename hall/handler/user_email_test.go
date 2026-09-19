package handler

import (
	"encoding/json"
	"testing"
)

func TestUpdateEmailArrayMarksAndDeletesOnlyTarget(t *testing.T) {
	current := `[{"id":1,"isRead":false},{"id":"2","isRead":false}]`
	read, err := updateEmailArray(current, "1", false)
	if err != nil {
		t.Fatal(err)
	}
	var readItems []map[string]any
	if err := json.Unmarshal([]byte(read), &readItems); err != nil {
		t.Fatal(err)
	}
	if len(readItems) != 2 || readItems[0]["isRead"] != true || readItems[1]["isRead"] != false {
		t.Fatalf("read result = %#v", readItems)
	}

	deleted, err := updateEmailArray(read, "2", true)
	if err != nil {
		t.Fatal(err)
	}
	var deletedItems []map[string]any
	if err := json.Unmarshal([]byte(deleted), &deletedItems); err != nil {
		t.Fatal(err)
	}
	if len(deletedItems) != 1 || emailIDString(deletedItems[0]["id"]) != "1" {
		t.Fatalf("delete result = %#v", deletedItems)
	}
}
