package request

import (
	"encoding/json"
	"testing"
)

func TestRoomMessageDataAcceptsLegacyNumericAndTextChat(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload string
		want    any
	}{
		{name: "interaction", payload: `{"type":307,"data":{"toChairID":1,"msg":4}}`, want: 4},
		{name: "text", payload: `{"type":307,"data":{"toChairID":1,"msg":"4"}}`, want: "4"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var request RoomMessageReq
			if err := json.Unmarshal([]byte(test.payload), &request); err != nil {
				t.Fatalf("decode room chat: %v", err)
			}
			if got := request.Data.Msg.Value(); got != test.want {
				t.Fatalf("room chat value = %#v, want %#v", got, test.want)
			}
		})
	}
}
