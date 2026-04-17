package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	larkBaseURL       = "https://open.feishu.cn/open-apis"
	tokenEndpoint     = "/auth/v3/tenant_access_token/internal"
	sendMsgEndpoint   = "/im/v1/messages"
	patchMsgEndpoint  = "/im/v1/messages/%s"
)

// tokenCache caches a tenant_access_token along with its expiry.
type tokenCache struct {
	mu      sync.Mutex
	token   string
	expires time.Time
}

func (c *tokenCache) get() (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" || time.Now().After(c.expires) {
		return "", false
	}
	return c.token, true
}

func (c *tokenCache) set(token string, ttlSeconds int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
	// Refresh 60 seconds before actual expiry.
	c.expires = time.Now().Add(time.Duration(ttlSeconds-60) * time.Second)
}

// BotClient wraps Lark Bot API calls (auth, send, patch messages).
type BotClient struct {
	httpClient *http.Client
	appID      string
	appSecret  string
	tokenCache tokenCache
}

// NewBotClient creates a new BotClient.
func NewBotClient(appID, appSecret string) *BotClient {
	return &BotClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		appID:      appID,
		appSecret:  appSecret,
	}
}

// getTenantAccessToken returns a valid tenant_access_token, fetching a new one if needed.
func (c *BotClient) getTenantAccessToken(ctx context.Context) (string, error) {
	if tok, ok := c.tokenCache.get(); ok {
		return tok, nil
	}

	body, _ := json.Marshal(map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		larkBaseURL+tokenEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get tenant_access_token: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("lark auth error code=%d msg=%s", result.Code, result.Msg)
	}

	c.tokenCache.set(result.TenantAccessToken, result.Expire)
	return result.TenantAccessToken, nil
}

// SendMessage sends a card message to a Lark group chat via Bot API.
// Returns the message_id which can be used to update the card later.
func (c *BotClient) SendMessage(ctx context.Context, chatID string, card *CardMessage) (string, error) {
	return c.sendCard(ctx, "chat_id", chatID, card)
}

// SendDirectMessage sends a card message directly to a user via Bot API.
// receiveIDType should be one of: "user_id", "open_id", "union_id", "email".
// Returns the message_id.
func (c *BotClient) SendDirectMessage(ctx context.Context, receiveIDType, receiveID string, card *CardMessage) (string, error) {
	switch receiveIDType {
	case "user_id", "open_id", "union_id", "email":
	default:
		return "", fmt.Errorf("unsupported receive_id_type for DM: %s", receiveIDType)
	}
	return c.sendCard(ctx, receiveIDType, receiveID, card)
}

// SendText sends a plain-text message via the Bot API (used for command replies).
// receiveIDType is typically "chat_id" for group replies or "open_id"/"user_id" for DMs.
func (c *BotClient) SendText(ctx context.Context, receiveIDType, receiveID, text string) (string, error) {
	textJSON, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", fmt.Errorf("marshal text: %w", err)
	}
	return c.sendRaw(ctx, receiveIDType, receiveID, "text", string(textJSON))
}

// sendCard is the shared implementation backing SendMessage / SendDirectMessage.
func (c *BotClient) sendCard(ctx context.Context, receiveIDType, receiveID string, card *CardMessage) (string, error) {
	cardJSON, err := json.Marshal(card.Card)
	if err != nil {
		return "", fmt.Errorf("marshal card: %w", err)
	}
	return c.sendRaw(ctx, receiveIDType, receiveID, "interactive", string(cardJSON))
}

// sendRaw is the underlying message send helper.
func (c *BotClient) sendRaw(ctx context.Context, receiveIDType, receiveID, msgType, content string) (string, error) {
	token, err := c.getTenantAccessToken(ctx)
	if err != nil {
		return "", err
	}

	payload := map[string]string{
		"receive_id": receiveID,
		"msg_type":   msgType,
		"content":    content,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		larkBaseURL+sendMsgEndpoint+"?receive_id_type="+receiveIDType,
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send bot message: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse send response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("lark send error code=%d msg=%s", result.Code, result.Msg)
	}
	return result.Data.MessageID, nil
}

// UpdateMessage patches the content of an existing card message.
func (c *BotClient) UpdateMessage(ctx context.Context, messageID string, card *CardMessage) error {
	token, err := c.getTenantAccessToken(ctx)
	if err != nil {
		return err
	}

	cardJSON, err := json.Marshal(card.Card)
	if err != nil {
		return fmt.Errorf("marshal card: %w", err)
	}

	payload := map[string]string{
		"msg_type": "interactive",
		"content":  string(cardJSON),
	}
	body, _ := json.Marshal(payload)

	url := larkBaseURL + fmt.Sprintf(patchMsgEndpoint, messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update bot message: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse update response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("lark update error code=%d msg=%s", result.Code, result.Msg)
	}
	return nil
}
