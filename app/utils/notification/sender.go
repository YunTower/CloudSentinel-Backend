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
	// 这里允许测试时强制发送，即使 Enabled 为 false（只要传入了配置）
	// 但为了安全，还是检查一下基本字段
	if config.SMTP == "" || config.From == "" || config.To == "" {
		return fmt.Errorf("邮件配置不完整")
	}

	// 构建邮件内容
	msg := bytes.Buffer{}
	msg.WriteString(fmt.Sprintf("From: %s\r\n", config.From))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", config.To))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(content)

	addr := fmt.Sprintf("%s:%d", config.SMTP, config.Port)

	// 根据安全类型选择发送方式
	switch strings.ToUpper(config.Security) {
	case "SSL", "TLS":
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: config.SMTP})
		if err != nil {
			return fmt.Errorf("TLS连接失败: %v", err)
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, config.SMTP)
		if err != nil {
			return fmt.Errorf("创建SMTP客户端失败: %v", err)
		}
		defer c.Quit()

		if config.Password != "" {
			auth := smtp.PlainAuth("", config.From, config.Password, config.SMTP)
			if err = c.Auth(auth); err != nil {
				return fmt.Errorf("SMTP认证失败: %v", err)
			}
		}

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
			return fmt.Errorf("关闭数据写入器失败: %v", err)
		}
		return nil
	case "STARTTLS":
		auth := smtp.PlainAuth("", config.From, config.Password, config.SMTP)
		if config.Password == "" {
			auth = nil
		}
		return smtp.SendMail(addr, auth, config.From, []string{config.To}, msg.Bytes())
	case "NONE", "":
		auth := smtp.PlainAuth("", config.From, config.Password, config.SMTP)
		if config.Password == "" {
			auth = nil
		}
		return smtp.SendMail(addr, auth, config.From, []string{config.To}, msg.Bytes())
	default:
		return fmt.Errorf("不支持的安全类型: %s", config.Security)
	}
}

func SendWebhook(config WebhookConfig, content string) error {
	if config.Webhook == "" {
		return fmt.Errorf("webhook配置不完整")
	}

	// 构建消息体
	message := map[string]interface{}{
		"msgtype":  "text",
		"msg_type": "text",
		"text": map[string]interface{}{
			"content": content,
		},
		"content": map[string]interface{}{
			"text": content,
		},
	}

	// 处理提及用户
	if strings.Contains(config.Webhook, "open.feishu.cn") { // 飞书适配
		if config.Mentioned != "" && config.Mentioned != "@all" {
			// TODO: 还有问题，待优化
			userID := strings.Split(config.Mentioned, ",")
			for _, id := range userID {
				message["content"].(map[string]interface{})["text"] = "<at user_id=\"" + id + "\">fish</at>" + message["content"].(map[string]interface{})["text"].(string)
			}
		} else if config.Mentioned == "@all" {
			message["content"].(map[string]interface{})["text"] = "<at user_id=\"all\">所有人</at>" + message["content"].(map[string]interface{})["text"].(string)
		}
	} else {
		if config.Mentioned != "" && config.Mentioned != "@all" {
			userID := strings.Split(config.Mentioned, ",")
			for _, id := range userID {
				message["text"].(map[string]interface{})["mentioned_list"] = append(message["text"].(map[string]interface{})["mentioned_list"].([]string), id)
			}
		} else if config.Mentioned == "@all" {
			message["text"].(map[string]interface{})["content"] = content + "\n<@all>"
		}
	}

	// 序列化消息体
	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	// 使用 facades.Http 发送请求
	resp, err := facades.Http().
		WithHeaders(map[string]string{"Content-Type": "application/json"}).
		Post(config.Webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	fmt.Println(resp.Body())
	fmt.Println(err)

	if resp.Failed() {
		body, _ := resp.Body()
		return fmt.Errorf("webhook接口返回错误状态码: %d, Body: %s", resp.Status(), body)
	}

	return nil
}
