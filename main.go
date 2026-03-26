package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type UpdateResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

type Message struct {
	MessageID int  `json:"message_id"`
	Chat      Chat `json:"chat"`
	Text      string `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		fmt.Println("BOT_TOKEN is not set")
		os.Exit(1)
	}

	fmt.Println("Bot is running...")

	offset := 0

	for {
		updates, err := getUpdates(token, offset)
		if err != nil {
			fmt.Println("getUpdates error:", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, upd := range updates {
			offset = upd.UpdateID + 1

			if upd.Message.Text == "" {
				continue
			}

			fmt.Printf("Received message: %s\n", upd.Message.Text)

			var reply string
			switch upd.Message.Text {
			case "/start":
				reply = "Hello world 👋"
			default:
				reply = "You said: " + upd.Message.Text
			}

			if err := sendMessage(token, upd.Message.Chat.ID, reply); err != nil {
				fmt.Println("sendMessage error:", err)
			}
		}
	}
}

func getUpdates(token string, offset int) ([]Update, error) {
	endpoint := fmt.Sprintf(
		"https://api.telegram.org/bot%s/getUpdates?timeout=30&offset=%d",
		token,
		offset,
	)

	client := &http.Client{Timeout: 35 * time.Second}

	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result UpdateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if !result.OK {
		return nil, fmt.Errorf("telegram API returned not ok")
	}

	return result.Result, nil
}

func sendMessage(token string, chatID int64, text string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	values := url.Values{}
	values.Set("chat_id", fmt.Sprintf("%d", chatID))
	values.Set("text", text)

	resp, err := http.PostForm(endpoint, values)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	return nil
}
