package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Bot struct {
	token   string
	chatIDs []string
	client  *http.Client
}

func NewBot(token string, chatIDs []string) *Bot {
	return &Bot{
		token:   token,
		chatIDs: chatIDs,
		client:  &http.Client{Timeout: 45 * time.Second},
	}
}

func (b *Bot) Broadcast(text string) error {
	var firstErr error

	for _, chatID := range b.chatIDs {
		if err := b.send(chatID, text); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("chat %s: %w", chatID, err)
			}
		}
	}

	return firstErr
}

func (b *Bot) send(chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.token)

	payload := map[string]string{
		"chat_id": chatID,
		"text":    text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	resp, err := b.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sending: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("telegram API returned %d: %v", resp.StatusCode, result["description"])
	}

	return nil
}
