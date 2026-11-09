package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// TelegramInlineButton represents a tap-to-open button below a message
type TelegramInlineButton struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// TelegramReplyMarkup represents an inline keyboard container
type TelegramReplyMarkup struct {
	InlineKeyboard [][]TelegramInlineButton `json:"inline_keyboard,omitempty"`
}

// TelegramSendMessagePayload defines the POST body for /sendMessage
type TelegramSendMessagePayload struct {
	ChatID                string               `json:"chat_id"`
	Text                  string               `json:"text"`
	ParseMode             string               `json:"parse_mode"`
	DisableWebPagePreview bool                 `json:"disable_web_page_preview"`
	ReplyMarkup           *TelegramReplyMarkup `json:"reply_markup,omitempty"`
}

// TelegramBotResponse is the standard wrapper from api.telegram.org
type TelegramBotResponse struct {
	OK          bool            `json:"ok"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// TelegramBotUser represents the bot identity returned by getMe
type TelegramBotUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

var telegramHTTPClient = &http.Client{
	Timeout: 8 * time.Second,
}

// isTelegramValidButtonURL checks if a URL complies with Telegram Bot API InlineKeyboardButton rules:
// - Must use http, https, or tg scheme
// - Must not be localhost, 127.0.0.1, or local hostnames without a public domain dot
func isTelegramValidButtonURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "tg" {
		return true
	}
	if scheme != "http" && scheme != "https" {
		return false
	}
	hostname := strings.ToLower(u.Hostname())
	if hostname == "" || hostname == "localhost" || hostname == "127.0.0.1" || hostname == "0.0.0.0" {
		return false
	}
	// Telegram requires a valid domain with a dot (e.g. seriestkd.com, railway.app)
	if !strings.Contains(hostname, ".") {
		return false
	}
	return true
}

// sanitizeTelegramButtons purges buttons with invalid or localhost URLs so Telegram API does not reject the entire message
func sanitizeTelegramButtons(buttons [][]TelegramInlineButton) [][]TelegramInlineButton {
	if len(buttons) == 0 {
		return nil
	}
	var validRows [][]TelegramInlineButton
	for _, row := range buttons {
		var validRow []TelegramInlineButton
		for _, btn := range row {
			if isTelegramValidButtonURL(btn.URL) {
				validRow = append(validRow, btn)
			}
		}
		if len(validRow) > 0 {
			validRows = append(validRows, validRow)
		}
	}
	return validRows
}

// SendTelegramMessage sends an HTML-formatted message with optional buttons to the target chat
func SendTelegramMessage(botToken, chatID, htmlText string, buttons [][]TelegramInlineButton) error {
	token := strings.TrimSpace(botToken)
	target := strings.TrimSpace(chatID)
	if token == "" {
		return errors.New("telegram bot token is empty")
	}
	if target == "" {
		return errors.New("telegram chat ID is empty")
	}

	// Mock / simulated mode for testing environments
	if token == "mock" || token == "simulated" || strings.HasPrefix(token, "test-") {
		log.Printf("🤖 [SIMULATED TELEGRAM DISPATCH] To: %s | Text: %s", target, htmlText)
		return nil
	}

	payload := TelegramSendMessagePayload{
		ChatID:                target,
		Text:                  htmlText,
		ParseMode:             "HTML",
		DisableWebPagePreview: false,
	}

	// Filter out invalid or localhost URLs to comply with Telegram Bot API validation
	sanitizedButtons := sanitizeTelegramButtons(buttons)
	if len(sanitizedButtons) > 0 {
		payload.ReplyMarkup = &TelegramReplyMarkup{
			InlineKeyboard: sanitizedButtons,
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode telegram payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := telegramHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram api network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var tgResp TelegramBotResponse
	if err := json.Unmarshal(respBytes, &tgResp); err != nil {
		return fmt.Errorf("telegram returned invalid JSON (HTTP %d): %s", resp.StatusCode, string(respBytes))
	}

	if !tgResp.OK {
		return fmt.Errorf("telegram api error (%d): %s", tgResp.ErrorCode, tgResp.Description)
	}

	return nil
}

// TestTelegramConnection calls getMe to verify bot credentials, and optionally sends a test ping
func TestTelegramConnection(botToken, chatID string) (string, error) {
	token := strings.TrimSpace(botToken)
	if token == "" {
		return "", errors.New("please enter a Telegram Bot Token")
	}

	if token == "mock" || token == "simulated" || strings.HasPrefix(token, "test-") {
		return "SimulatedTKDBot", nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token)
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := telegramHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not reach Telegram API: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var tgResp TelegramBotResponse
	if err := json.Unmarshal(respBytes, &tgResp); err != nil {
		return "", fmt.Errorf("invalid response from Telegram: %s", string(respBytes))
	}

	if !tgResp.OK {
		return "", fmt.Errorf("bot authentication failed (%d): %s", tgResp.ErrorCode, tgResp.Description)
	}

	var botUser TelegramBotUser
	if err := json.Unmarshal(tgResp.Result, &botUser); err != nil {
		return "", fmt.Errorf("failed to parse bot info: %w", err)
	}

	// If chat ID is provided, send an immediate test greeting
	target := strings.TrimSpace(chatID)
	if target != "" {
		testMsg := fmt.Sprintf("🥋 <b>Series Taekwondo Bot Connected!</b>\n\n"+
			"The bot <b>@%s</b> is now active and ready to deliver announcements and alerts from STMS.\n\n"+
			"<i>Tested at %s</i>",
			botUser.Username, time.Now().Format("Jan 02, 2006 15:04 MST"))

		// Only attach Portal button if an external public base URL is configured
		var buttons [][]TelegramInlineButton
		baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
		if baseURL == "" {
			baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("HOST")), "/")
		}
		if isTelegramValidButtonURL(baseURL) {
			buttons = append(buttons, []TelegramInlineButton{
				{Text: "🥋 Open STMS Portal", URL: baseURL + "/dashboard"},
			})
		}

		if err := SendTelegramMessage(token, target, testMsg, buttons); err != nil {
			return botUser.Username, fmt.Errorf("bot verified (@%s), but test message to %s failed: %w", botUser.Username, target, err)
		}
	}

	return botUser.Username, nil
}

