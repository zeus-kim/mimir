package delivery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/smtp"
	"strings"
)

type Channel interface {
	Send(title, body string, audio []byte) error
	Name() string
}

// Telegram Bot API
type TelegramChannel struct {
	BotToken string
	ChatID   string
}

func (t *TelegramChannel) Name() string { return "telegram" }

func (t *TelegramChannel) Send(title, body string, audio []byte) error {
	text := fmt.Sprintf("*%s*\n\n%s", title, body)

	// Send text message
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	payload := map[string]interface{}{
		"chat_id":    t.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	resp.Body.Close()

	// Send audio if provided
	if len(audio) > 0 {
		return t.sendAudio(audio)
	}

	return nil
}

func (t *TelegramChannel) sendAudio(audio []byte) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendVoice", t.BotToken)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("chat_id", t.ChatID)

	fw, _ := w.CreateFormFile("voice", "briefing.mp3")
	fw.Write(audio)
	w.Close()

	resp, err := http.Post(url, w.FormDataContentType(), &buf)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// Slack Webhook
type SlackChannel struct {
	WebhookURL string
}

func (s *SlackChannel) Name() string { return "slack" }

func (s *SlackChannel) Send(title, body string, audio []byte) error {
	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{"type": "plain_text", "text": title},
			},
			{
				"type": "section",
				"text": map[string]string{"type": "mrkdwn", "text": body},
			},
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(s.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// Discord Webhook
type DiscordChannel struct {
	WebhookURL string
}

func (d *DiscordChannel) Name() string { return "discord" }

func (d *DiscordChannel) Send(title, body string, audio []byte) error {
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": body,
				"color":       3447003, // Blue
			},
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(d.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// Email via SMTP
type EmailChannel struct {
	SMTPHost string
	SMTPPort int
	Username string
	Password string
	From     string
	To       []string
}

func (e *EmailChannel) Name() string { return "email" }

func (e *EmailChannel) Send(title, body string, audio []byte) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		e.From,
		strings.Join(e.To, ","),
		title,
		body,
	)

	auth := smtp.PlainAuth("", e.Username, e.Password, e.SMTPHost)
	addr := fmt.Sprintf("%s:%d", e.SMTPHost, e.SMTPPort)

	return smtp.SendMail(addr, auth, e.From, e.To, []byte(msg))
}

// ntfy.sh (self-hosted or public)
type NtfyChannel struct {
	ServerURL string
	Topic     string
}

func (n *NtfyChannel) Name() string { return "ntfy" }

func (n *NtfyChannel) Send(title, body string, audio []byte) error {
	url := fmt.Sprintf("%s/%s", n.ServerURL, n.Topic)

	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Title", title)
	req.Header.Set("Priority", "default")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// External webhook (e.g., meshclaw on remote machine)
type WebhookChannel struct {
	URL     string
	Method  string
	Headers map[string]string
}

func (w *WebhookChannel) Name() string { return "webhook" }

func (w *WebhookChannel) Send(title, body string, audio []byte) error {
	method := w.Method
	if method == "" {
		method = "POST"
	}

	payload := map[string]interface{}{
		"title": title,
		"body":  body,
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest(method, w.URL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return nil
}

// MultiChannel sends to multiple channels
type MultiChannel struct {
	Channels []Channel
}

func (m *MultiChannel) Send(title, body string, audio []byte) error {
	var errs []string
	for _, ch := range m.Channels {
		if err := ch.Send(title, body, audio); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ch.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("delivery errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
