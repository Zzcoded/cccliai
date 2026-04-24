package channels

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cccliai/app/internal/agent"
	"github.com/cccliai/app/internal/config"
)

type FeishuConfig struct {
	AppID             string `json:"appId"`
	AppSecret         string `json:"appSecret"`
	VerificationToken string `json:"verificationToken"`
}

type FeishuChannel struct {
	ID     string
	Config FeishuConfig
}

type FeishuManager struct {
	db     *sql.DB
	config *config.Config
}

type feishuWebhookPayload struct {
	Type      string             `json:"type"`
	Token     string             `json:"token"`
	Challenge string             `json:"challenge"`
	Encrypt   string             `json:"encrypt"`
	Header    feishuEventHeader  `json:"header"`
	Event     feishuMessageEvent `json:"event"`
}

type feishuEventHeader struct {
	EventType string `json:"event_type"`
	Token     string `json:"token"`
	AppID     string `json:"app_id"`
}

type feishuMessageEvent struct {
	Sender  feishuSender  `json:"sender"`
	Message feishuMessage `json:"message"`
}

type feishuSender struct {
	SenderType string `json:"sender_type"`
}

type feishuMessage struct {
	ChatID      string `json:"chat_id"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
}

func NewFeishuManager(db *sql.DB, cfg *config.Config) *FeishuManager {
	return &FeishuManager{db: db, config: cfg}
}

func (fm *FeishuManager) LoadActiveChannels() ([]FeishuChannel, error) {
	rows, err := fm.db.Query("SELECT id, config FROM channels WHERE type = 'feishu' AND enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []FeishuChannel
	for rows.Next() {
		var channel FeishuChannel
		var configStr sql.NullString
		if err := rows.Scan(&channel.ID, &configStr); err != nil {
			return nil, err
		}

		if configStr.Valid && configStr.String != "" {
			if err := json.Unmarshal([]byte(configStr.String), &channel.Config); err != nil {
				log.Printf("Error unmarshaling feishu config for %s: %v", channel.ID, err)
				continue
			}
		}
		channels = append(channels, channel)
	}

	return channels, rows.Err()
}

func (fm *FeishuManager) HandleWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		activeChannels, err := fm.LoadActiveChannels()
		if err != nil {
			http.Error(w, "Failed to load feishu channels", http.StatusInternalServerError)
			return
		}
		if len(activeChannels) == 0 {
			http.Error(w, "No active feishu channels", http.StatusNotFound)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var payload feishuWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Current implementation supports plaintext callbacks only.
		if strings.TrimSpace(payload.Encrypt) != "" {
			log.Printf("Feishu webhook received encrypted payload; please set callback encryption to plaintext")
			http.Error(w, "Encrypted callback body is not supported yet. Set Feishu callback encryption to plaintext.", http.StatusBadRequest)
			return
		}

		// URL verification handshake.
		if payload.Type == "url_verification" && payload.Challenge != "" {
			if !matchesFeishuToken(activeChannels, payload.Token) {
				http.Error(w, "Invalid verification token", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"challenge": payload.Challenge})
			return
		}

		channel := selectFeishuChannel(activeChannels, payload.Header.AppID, payload.Header.Token)
		if channel == nil {
			log.Printf("No matching feishu channel: app_id=%s token_present=%t", payload.Header.AppID, strings.TrimSpace(payload.Header.Token) != "")
			http.Error(w, "No matching feishu channel config", http.StatusUnauthorized)
			return
		}

		if payload.Header.EventType != "im.message.receive_v1" {
			log.Printf("Feishu webhook ignored event_type=%s", payload.Header.EventType)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]int{"code": 0})
			return
		}

		// Avoid bot loops.
		if payload.Event.Sender.SenderType == "app" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]int{"code": 0})
			return
		}

		userText, ok := parseFeishuText(payload.Event.Message.MessageType, payload.Event.Message.Content)
		if !ok || strings.TrimSpace(userText) == "" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]int{"code": 0})
			return
		}

		chatID := payload.Event.Message.ChatID
		channelConfig := *channel
		go fm.processFeishuMessage(channelConfig, chatID, userText)

		// Ack immediately so Feishu won't keep retrying.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"code": 0})
	}
}

func (fm *FeishuManager) processFeishuMessage(channel FeishuChannel, chatID string, userText string) {
	replyText := ""
	p := getFallbackProvider()
	if p == nil {
		replyText = "No provider key configured. Set DEEPSEEK_API_KEY or PROVIDER_API_KEY."
	} else {
		loop := agent.NewCognitionLoop(p, 25, fm.config)
		runCtx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()

		resp, err := loop.Run(runCtx, userText)
		if err != nil {
			replyText = fmt.Sprintf("AI error: %v", err)
		} else {
			replyText = resp
		}
	}

	if err := sendFeishuTextMessage(channel.Config, chatID, replyText); err != nil {
		log.Printf("Feishu reply failed: %v", err)
	}
}

func parseFeishuText(messageType string, content string) (string, bool) {
	if messageType != "text" {
		return "", false
	}

	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return "", false
	}
	return payload.Text, true
}

func matchesFeishuToken(channels []FeishuChannel, token string) bool {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return true
	}
	for _, ch := range channels {
		if strings.TrimSpace(ch.Config.VerificationToken) == trimmed {
			return true
		}
	}
	return false
}

func selectFeishuChannel(channels []FeishuChannel, appID string, token string) *FeishuChannel {
	for i := range channels {
		ch := &channels[i]
		if ch.Config.AppID != "" && ch.Config.AppID == appID {
			if ch.Config.VerificationToken == "" || ch.Config.VerificationToken == token {
				return ch
			}
		}
	}

	for i := range channels {
		ch := &channels[i]
		if ch.Config.VerificationToken != "" && ch.Config.VerificationToken == token {
			return ch
		}
	}
	return nil
}

func sendFeishuTextMessage(cfg FeishuConfig, chatID string, text string) error {
	token, err := getFeishuTenantAccessToken(cfg)
	if err != nil {
		return err
	}

	contentJSON, _ := json.Marshal(map[string]string{"text": text})
	bodyBytes, _ := json.Marshal(map[string]string{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(contentJSON),
	})

	req, _ := http.NewRequest(http.MethodPost, "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var apiResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(body, &apiResp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || apiResp.Code != 0 {
		return fmt.Errorf("feishu send failed: http=%d code=%d msg=%s body=%s", resp.StatusCode, apiResp.Code, apiResp.Msg, string(body))
	}
	return nil
}

func getFeishuTenantAccessToken(cfg FeishuConfig) (string, error) {
	bodyBytes, _ := json.Marshal(map[string]string{
		"app_id":     cfg.AppID,
		"app_secret": cfg.AppSecret,
	})

	req, _ := http.NewRequest(http.MethodPost, "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenResp struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.Code != 0 || tokenResp.TenantAccessToken == "" {
		return "", fmt.Errorf("feishu auth failed: code=%d msg=%s body=%s", tokenResp.Code, tokenResp.Msg, string(body))
	}
	return tokenResp.TenantAccessToken, nil
}
