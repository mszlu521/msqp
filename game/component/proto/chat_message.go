package proto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// ChatMessage accepts both free text and the numeric emotion index used by
// the legacy Cocos client. Value preserves the original JSON value kind.
type ChatMessage struct {
	text    string
	numeric bool
}

func (m *ChatMessage) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return fmt.Errorf("chat message must be a string or number")
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		m.text = text
		m.numeric = false
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var number json.Number
	if err := decoder.Decode(&number); err != nil || number == "" {
		return fmt.Errorf("chat message must be a string or number")
	}
	if _, err := strconv.Atoi(number.String()); err != nil {
		return fmt.Errorf("chat emotion must be an integer: %w", err)
	}
	m.text = number.String()
	m.numeric = true
	return nil
}

func (m ChatMessage) Value() any {
	if m.numeric {
		value, _ := strconv.Atoi(m.text)
		return value
	}
	return m.text
}
