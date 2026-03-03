package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type CommandHandler func() (string, error)

type update struct {
	UpdateID int      `json:"update-id"`
	Message  *message `json:"message"`
}

type message struct {
	Chat chat   `json:"chat"`
	Text string `json:"text"`
}

type chat struct {
	ID int64 `json:"id"`
}

type getUpdatesResponse struct {
	OK     bool     `json:"ok"`
	Result []update `json:"result"`
}

func (b *Bot) ListenForCommands(ctx context.Context, commands map[string]CommandHandler) {
	offset := b.drainOldUpdates()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		upds, err := b.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("telegram: getUpdates error: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, upd := range upds {
			offset = upd.UpdateID + 1

			if upd.Message == nil || upd.Message.Text == "" {
				continue
			}

			cmd := upd.Message.Text
			if !strings.HasPrefix(cmd, "/") {
				continue
			}
			cmd = strings.TrimPrefix(cmd, "/")
			cmd = strings.SplitN(cmd, "@", 2)[0]
			cmd = strings.TrimSpace(cmd)

			handler, ok := commands[cmd]
			if !ok {
				continue
			}

			chatID := fmt.Sprintf("%d", upd.Message.Chat.ID)
			response, err := handler()
			if err != nil {
				log.Printf("telegram: command /%s error: %v", cmd, err)
				b.send(chatID, fmt.Sprintf("error: %v", err))
				continue
			}

			if err := b.send(chatID, response); err != nil {
				log.Printf("telegram: failed to send /%s response: %v", cmd, err)
			}
		}
	}
}

func (b *Bot) getUpdates(ctx context.Context, offset int) ([]update, error) {
	url := fmt.Sprintf(
		"https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30",
		b.token, offset,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result getUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("getUpdates returned ok=false")
	}

	return result.Result, nil
}

func (b *Bot) drainOldUpdates() int {
	url := fmt.Sprintf(
		"https://api.telegram.org/bot%s/getUpdates?offset=-1&timeout=0",
		b.token,
	)

	resp, err := b.client.Get(url)
	if err != nil {
		log.Printf("draining old updates failed: %v", err)
		return 0
	}
	defer resp.Body.Close()

	var result getUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0
	}

	if len(result.Result) > 0 {
		last := result.Result[len(result.Result)-1]
		log.Printf("skipped %d old telegram updates", last.UpdateID)
		return last.UpdateID + 1
	}

	return 0
}
