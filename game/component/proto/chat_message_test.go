package proto

import (
	"encoding/json"
	"testing"
)

func TestChatMessageAcceptsLegacyEmotionAndText(t *testing.T) {
	var emotion ChatMessage
	if err := json.Unmarshal([]byte(`12`), &emotion); err != nil {
		t.Fatalf("decode numeric emotion: %v", err)
	}
	if value, ok := emotion.Value().(int); !ok || value != 12 {
		t.Fatalf("numeric emotion was not preserved: %#v", emotion.Value())
	}

	var text ChatMessage
	if err := json.Unmarshal([]byte(`"12"`), &text); err != nil {
		t.Fatalf("decode text: %v", err)
	}
	if value, ok := text.Value().(string); !ok || value != "12" {
		t.Fatalf("text message was not preserved: %#v", text.Value())
	}
}

func TestChatMessageRejectsUnsupportedValues(t *testing.T) {
	for _, value := range []string{`true`, `null`, `1.5`} {
		var message ChatMessage
		if err := json.Unmarshal([]byte(value), &message); err == nil {
			t.Fatalf("accepted unsupported chat message %s", value)
		}
	}
}
