// Package tg is a small Telegram Bot API client (only what this project needs).
package tg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Button struct {
	Text         string `json:"text"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
}

type Keyboard [][]Button

type SendOpts struct {
	Keyboard Keyboard
}

// API is the subset of the Bot API used by the app. Tests provide a fake.
type API interface {
	GetMe(ctx context.Context) (*User, error)
	SendMessage(ctx context.Context, chatID int64, html string, opts *SendOpts) (int64, error)
	EditMessage(ctx context.Context, chatID, messageID int64, html string, opts *SendOpts) error
	AnswerCallback(ctx context.Context, id, text string, alert bool) error
	GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error)
	SetWebhook(ctx context.Context, url, secret string) error
	DeleteWebhook(ctx context.Context) error
}

type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string { return fmt.Sprintf("telegram: %d %s", e.Code, e.Description) }

// Blocked reports that the user blocked the bot or deactivated their account.
func (e *APIError) Blocked() bool { return e.Code == 403 }

type Client struct {
	token string
	base  string
	http  *http.Client
}

func NewClient(token string) *Client {
	return &Client{token: token, base: "https://api.telegram.org", http: &http.Client{}}
}

// WithBaseURL points the client at another server (tests).
func (c *Client) WithBaseURL(u string) *Client { c.base = u; return c }

func (c *Client) call(ctx context.Context, timeout time.Duration, method string, payload, out any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return redact(err, c.token)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var env struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("telegram %s: bad response (HTTP %d)", method, resp.StatusCode)
	}
	if !env.OK {
		return &APIError{Code: env.ErrorCode, Description: env.Description}
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// redact keeps the bot token out of logged transport errors (it is part of the URL).
func redact(err error, token string) error {
	msg := bytes.ReplaceAll([]byte(err.Error()), []byte(token), []byte("<token>"))
	return fmt.Errorf("%s", msg)
}

func (c *Client) GetMe(ctx context.Context) (*User, error) {
	var u User
	if err := c.call(ctx, 15*time.Second, "getMe", struct{}{}, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func markup(opts *SendOpts) any {
	if opts == nil || len(opts.Keyboard) == 0 {
		return nil
	}
	return map[string]any{"inline_keyboard": opts.Keyboard}
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, html string, opts *SendOpts) (int64, error) {
	var m Message
	err := c.call(ctx, 15*time.Second, "sendMessage", map[string]any{
		"chat_id": chatID, "text": html, "parse_mode": "HTML", "disable_web_page_preview": true, "reply_markup": markup(opts),
	}, &m)
	return m.MessageID, err
}

func (c *Client) EditMessage(ctx context.Context, chatID, messageID int64, html string, opts *SendOpts) error {
	return c.call(ctx, 15*time.Second, "editMessageText", map[string]any{
		"chat_id": chatID, "message_id": messageID, "text": html, "parse_mode": "HTML",
		"disable_web_page_preview": true, "reply_markup": markupOrEmpty(opts),
	}, nil)
}

// markupOrEmpty sends an explicit empty keyboard so buttons can be removed on edit.
func markupOrEmpty(opts *SendOpts) any {
	if m := markup(opts); m != nil {
		return m
	}
	return map[string]any{"inline_keyboard": [][]Button{}}
}

func (c *Client) AnswerCallback(ctx context.Context, id, text string, alert bool) error {
	return c.call(ctx, 15*time.Second, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text, "show_alert": alert}, nil)
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	var ups []Update
	err := c.call(ctx, time.Duration(timeoutSec+15)*time.Second, "getUpdates", map[string]any{
		"offset": offset, "timeout": timeoutSec, "allowed_updates": []string{"message", "callback_query"},
	}, &ups)
	return ups, err
}

func (c *Client) SetWebhook(ctx context.Context, url, secret string) error {
	return c.call(ctx, 15*time.Second, "setWebhook", map[string]any{
		"url": url, "secret_token": secret, "allowed_updates": []string{"message", "callback_query"}, "drop_pending_updates": true,
	}, nil)
}

func (c *Client) DeleteWebhook(ctx context.Context) error {
	return c.call(ctx, 15*time.Second, "deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}
