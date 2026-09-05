// Package notify 通知器：企业微信 / Server酱 / Telegram / 自定义 webhook 四渠道
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Event 通知事件类型
type Event string

const (
	EventSuccess       Event = "success"
	EventFailed        Event = "failed"
	EventCookieInvalid Event = "cookie_invalid"
)

// Notifier 分发通知到已配置渠道
type Notifier struct {
	getSetting func(key string) (string, error)
	client     *http.Client
}

func New(getSetting func(string) (string, error)) *Notifier {
	return &Notifier{
		getSetting: getSetting,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send 按事件过滤后分发到各渠道；单渠道失败不影响其他渠道
func (n *Notifier) Send(ev Event, title, content string) {
	eventsCfg, _ := n.getSetting("notify.events")
	if eventsCfg != "" && !containsEvent(eventsCfg, string(ev)) {
		return
	}
	go func() {
		if v, _ := n.getSetting("notify.wecom_webhook"); v != "" {
			n.sendWecom(v, title, content)
		}
		if v, _ := n.getSetting("notify.serverchan_sendkey"); v != "" {
			n.sendServerChan(v, title, content)
		}
		token, _ := n.getSetting("notify.telegram_bot_token")
		chatID, _ := n.getSetting("notify.telegram_chat_id")
		if token != "" && chatID != "" {
			n.sendTelegram(token, chatID, title+"\n"+content)
		}
		if v, _ := n.getSetting("notify.custom_webhook"); v != "" {
			n.sendCustom(v, title, content)
		}
	}()
}

func containsEvent(cfg, ev string) bool {
	for _, s := range strings.Split(cfg, ",") {
		if strings.TrimSpace(s) == ev {
			return true
		}
	}
	return false
}

func (n *Notifier) postJSON(url string, body any) error {
	b, _ := json.Marshal(body)
	resp, err := n.client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return nil
}

// sendWecom 企业微信机器人
func (n *Notifier) sendWecom(webhook, title, content string) {
	err := n.postJSON(webhook, map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": title + "\n" + content},
	})
	if err != nil {
		log.Printf("[notify] wecom 失败: %v", err)
	}
}

// sendServerChan Server酱 (sctapi.ftqq.com)
func (n *Notifier) sendServerChan(sendkey, title, content string) {
	form := url.Values{}
	form.Set("title", title)
	form.Set("desp", content)
	resp, err := n.client.Post("https://sctapi.ftqq.com/"+sendkey+".send",
		"application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		log.Printf("[notify] serverchan 失败: %v", err)
		return
	}
	resp.Body.Close()
}

// sendTelegram Telegram Bot API
func (n *Notifier) sendTelegram(token, chatID, text string) {
	err := n.postJSON("https://api.telegram.org/bot"+token+"/sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	if err != nil {
		log.Printf("[notify] telegram 失败: %v", err)
	}
}

// sendCustom 自定义 webhook：POST JSON {title, content, time}
func (n *Notifier) sendCustom(webhook, title, content string) {
	err := n.postJSON(webhook, map[string]any{
		"title":   title,
		"content": content,
		"time":    time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("[notify] custom 失败: %v", err)
	}
}

// SendTest 发送测试通知（设置页用）
func (n *Notifier) SendTest() {
	n.Send(EventSuccess, "baidu-auto-save 测试通知", "如果你看到这条消息，说明通知渠道配置成功。")
}
