package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
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
	MessageID int             `json:"message_id"`
	Chat      Chat            `json:"chat"`
	Text      string          `json:"text"`
	Entities  []MessageEntity `json:"entities"`
}

type MessageEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"` // UTF-16 code units
	Length int    `json:"length"` // UTF-16 code units
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // "private", "group", "supergroup", ...
}

type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func main() {
	botToken := os.Getenv("BOT_TOKEN")
	geminiKey := os.Getenv("GEMINI_API_KEY")

	if botToken == "" {
		fmt.Println("BOT_TOKEN is not set")
		os.Exit(1)
	}
	if geminiKey == "" {
		fmt.Println("GEMINI_API_KEY is not set")
		os.Exit(1)
	}

	botUsername := "@SeniorMouravi_bot"

	fmt.Println("Bot is running...")

	offset := 0

	for {
		updates, err := getUpdates(botToken, offset)
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

			text := strings.TrimSpace(upd.Message.Text)
			fmt.Printf("chatType=%s text=%q\n", upd.Message.Chat.Type, text)

			var prompt string

			switch upd.Message.Chat.Type {
			case "private":
				// In DM: answer every text message
				prompt = text

			case "group", "supergroup":
				// In groups: answer only if mentioned
				if !isMentioned(upd.Message, botUsername) {
					fmt.Println("group message ignored: bot not mentioned")
					continue
				}

				prompt = removeBotMention(upd.Message.Text, upd.Message.Entities, botUsername)
				prompt = strings.TrimSpace(prompt)
				if prompt == "" {
					prompt = "Hello 👋 Ask me anything."
				}

			default:
				continue
			}

			if text == "/start" || text == "/start@SeniorMouravi_bot" {
				prompt = "Hello 👋 Ask me anything."
			}

			reply, err := askGemini(geminiKey, prompt)
			if err != nil {
				fmt.Println("askGemini error:", err)
				reply = "Sorry, I could not get a response."
			}

			reply = limitChars(reply, 1024)

			if err := sendMessage(botToken, upd.Message.Chat.ID, upd.Message.MessageID, reply); err != nil {
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

func askGemini(apiKey, userText string) (string, error) {
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=" + url.QueryEscape(apiKey)

	prompt := "Answer clearly and briefly in plain text. Maximum 1024 characters.\n\nUser: " + userText

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 40 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("gemini API error: %s", string(body))
	}

	var result GeminiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty Gemini response")
	}

	return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
}

func sendMessage(token string, chatID int64, replyToMessageID int, text string) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	values := url.Values{}
	values.Set("chat_id", fmt.Sprintf("%d", chatID))
	values.Set("text", text)
	values.Set("reply_to_message_id", fmt.Sprintf("%d", replyToMessageID))

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

func isMentioned(msg Message, botUsername string) bool {
	lowerBot := strings.ToLower(botUsername)

	fmt.Println(lowerBot)

	for _, e := range msg.Entities {
		if e.Type != "mention" {
			continue
		}
		part := entityText(msg.Text, e.Offset, e.Length)
		if strings.ToLower(part) == lowerBot {
			return true
		}
	}

	// fallback
	return strings.Contains(strings.ToLower(msg.Text), lowerBot)
}

func removeBotMention(text string, entities []MessageEntity, botUsername string) string {
	out := text
	lowerBot := strings.ToLower(botUsername)

	for _, e := range entities {
		if e.Type != "mention" {
			continue
		}
		part := entityText(out, e.Offset, e.Length)
		if strings.ToLower(part) == lowerBot {
			out = strings.Replace(out, part, "", 1)
			break
		}
	}

	out = strings.ReplaceAll(out, botUsername, "")
	out = strings.ReplaceAll(out, strings.TrimPrefix(botUsername, "@")+":", "")
	out = strings.TrimSpace(out)
	out = strings.TrimLeft(out, ":,- ")
	return out
}

func entityText(s string, offsetUTF16, lengthUTF16 int) string {
	runes := []rune(s)
	u16 := utf16.Encode(runes)

	if offsetUTF16 < 0 || lengthUTF16 < 0 || offsetUTF16 > len(u16) {
		return ""
	}

	end := offsetUTF16 + lengthUTF16
	if end > len(u16) {
		end = len(u16)
	}

	decoded := utf16.Decode(u16[offsetUTF16:end])
	return string(decoded)
}

func limitChars(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}

	runes := []rune(s)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}
