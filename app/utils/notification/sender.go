package notification

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/goravel/framework/facades"
)

// EmailConfig 邮件配置
type EmailConfig struct {
	Enabled  bool   `json:"enabled"`
	SMTP     string `json:"smtp"`
	Port     int    `json:"port"`
	Security string `json:"security"` // NONE, STARTTLS, SSL
	From     string `json:"from"`
	To       string `json:"to"`
	Password string `json:"password"`
}

// WebhookConfig Webhook配置
type WebhookConfig struct {
	Enabled   bool   `json:"enabled"`
	Webhook   string `json:"webhook"`
	Mentioned string `json:"mentioned"`
}

func SendEmail(config EmailConfig, subject, content string) error {
	if config.SMTP == "" || config.From == "" || config.To == "" {
		return fmt.Errorf("邮件配置不完整")
	}

	msg := bytes.Buffer{}
	msg.WriteString(fmt.Sprintf("From: %s\r\n", config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", config.To))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))

	// 根据内容自动判断是 HTML 还是纯文本
	contentType := "text/plain; charset=UTF-8"
	if strings.HasPrefix(strings.TrimSpace(content), "<") {
		// 如果内容以 HTML 标签开头，则认为是 HTML
		contentType = "text/html; charset=UTF-8"
	}

	msg.WriteString(fmt.Sprintf("Content-Type: %s\r\n", contentType))
	msg.WriteString("\r\n")
	msg.WriteString(content)

	addr := fmt.Sprintf("%s:%d", config.SMTP, config.Port)
	var c *smtp.Client
	var err error

	// 建立连接
	if strings.ToUpper(config.Security) == "SSL" || strings.ToUpper(config.Security) == "TLS" {
		// SSL/TLS 模式
		tlsConfig := &tls.Config{
			ServerName:         config.SMTP,
			InsecureSkipVerify: true, // 允许自签名证书
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			if strings.Contains(err.Error(), "first record does not look like a TLS handshake") {
				return fmt.Errorf("TLS连接失败: 端口响应非TLS数据，请检查端口(通常465为SSL/TLS)或尝试STARTTLS模式: %v", err)
			}
			return fmt.Errorf("TLS连接失败: %v", err)
		}
		c, err = smtp.NewClient(conn, config.SMTP)
	} else {
		// STARTTLS 或 明文模式
		c, err = smtp.Dial(addr)
	}

	if err != nil {
		return fmt.Errorf("连接SMTP服务器失败: %v", err)
	}

	defer func() {
		_ = c.Quit()
		_ = c.Close()
	}()

	// STARTTLS 升级
	if strings.ToUpper(config.Security) == "STARTTLS" {
		tlsConfig := &tls.Config{
			ServerName:         config.SMTP,
			InsecureSkipVerify: true,
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS升级失败: %v", err)
		}
	}

	// 认证
	if config.Password != "" {
		auth := smtp.PlainAuth("", config.From, config.Password, config.SMTP)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("SMTP认证失败: %v", err)
		}
	}

	// 发送邮件
	if err = c.Mail(config.From); err != nil {
		return fmt.Errorf("设置发件人失败: %v", err)
	}
	if err = c.Rcpt(config.To); err != nil {
		return fmt.Errorf("设置收件人失败: %v", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("创建数据写入器失败: %v", err)
	}

	_, err = w.Write(msg.Bytes())
	if err != nil {
		return fmt.Errorf("写入邮件内容失败: %v", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("发送邮件数据失败: %v", err)
	}

	return nil
}

func SendWebhook(config WebhookConfig, content string) error {
	if config.Webhook == "" {
		return fmt.Errorf("webhook配置不完整")
	}

	var message map[string]interface{}

	if strings.Contains(config.Webhook, "open.feishu.cn") {
		// 飞书
		// 结构: {"msg_type": "text", "content": {"text": "..."}}
		text := content

		if config.Mentioned == "@all" {
			text += "\n<at user_id=\"all\">所有人</at>"
		} else if config.Mentioned != "" {
			userIDs := strings.Split(config.Mentioned, ",")
			for _, id := range userIDs {
				id = strings.TrimSpace(id)
				if id != "" {
					text += fmt.Sprintf("<at user_id=\"%s\"></at>", id)
				}
			}
		}

		message = map[string]interface{}{
			"msg_type": "text",
			"content": map[string]interface{}{
				"text": text,
			},
		}
	} else {
		// 企业微信 / 通用
		// 结构: {"msgtype": "text", "text": {"content": "...", "mentioned_list": [...]}}
		message = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]interface{}{
				"content": content,
			},
		}

		if config.Mentioned == "@all" {
			// 兼容处理
			message["text"].(map[string]interface{})["mentioned_list"] = []string{"@all"}
			message["text"].(map[string]interface{})["content"] = content + "\n@all"
		} else if config.Mentioned != "" {
			userIDs := strings.Split(config.Mentioned, ",")
			trimmedIDs := make([]string, 0, len(userIDs))
			for _, id := range userIDs {
				id = strings.TrimSpace(id)
				if id != "" {
					trimmedIDs = append(trimmedIDs, id)
				}
			}
			if len(trimmedIDs) > 0 {
				message["text"].(map[string]interface{})["mentioned_list"] = trimmedIDs
			}
		}
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	resp, err := facades.Http().
		WithHeaders(map[string]string{"Content-Type": "application/json"}).
		Post(config.Webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	if resp.Failed() {
		body, _ := resp.Body()
		return fmt.Errorf("webhook接口返回错误状态码: %d, Body: %s", resp.Status(), body)
	}

	return nil
}
