package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"goravel/app/cryptoutil"
	wspanelkey "goravel/app/services/websocket/panelkey"
	"goravel/app/repositories"
)

// Agent 下发命令签名。
//
// 面板与 Agent 之间的控制通道（WS 明文、HTTP 任务轮询）缺乏传输层保证时，
// 伪造的 service_check / restart / update_config / update 命令可以让 Agent
// 成为攻击跳板或被篡改配置。所有下发命令都附带面板 RSA 私钥的签名
// （覆盖命令 data 的规范化 JSON + command_id + 时间戳），Agent 端用
// key_exchange 阶段获得的面板公钥验签，验签失败一律拒绝执行。
//
// 密钥对由 websocket/panelkey 统一管理（与密钥交换共用同一份，
// 生成路径加锁避免并发双生成互相覆盖）。

// canonicalAgentCommandJSON 生成命令数据的规范化 JSON。
// encoding/json 对 map 按 key 排序输出，数值统一为 float64 最短表示，
// HTML 转义（<>& → \u003c 等）与 U+2028/2029 转义行为一致。
// ⚠ 与 Agent 端 agent/internal/reporter/command_signature.go 的
// canonicalAgentCommandJSON 必须逐字节一致：两端都使用标准库 encoding/json
// 的 map 序列化；任何一端更换 JSON 库或关闭 HTML 转义都会导致全线验签
// 失败（两端互为锚点，改动前先跑两端的一致性测试）。
func canonicalAgentCommandJSON(data map[string]interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// SignAgentCommand 对命令数据签名，返回 base64 签名与 Unix 秒级时间戳。
func SignAgentCommand(data map[string]interface{}, commandID string) (string, int64, error) {
	privateKey, _, err := wspanelkey.GetOrGenerate()
	if err != nil {
		return "", 0, err
	}
	ts := time.Now().Unix()
	canonical, err := canonicalAgentCommandJSON(data)
	if err != nil {
		return "", 0, fmt.Errorf("序列化命令数据失败: %w", err)
	}
	signed := append(canonical, []byte(fmt.Sprintf("|%s|%d", commandID, ts))...)
	signature, err := cryptoutil.SignData(signed, privateKey)
	if err != nil {
		return "", 0, fmt.Errorf("命令签名失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), ts, nil
}

// VerifyAgentCommandSignature 在 Agent 上报的 payload 中校验签名（服务端辅助测试用）。
func VerifyAgentCommandSignature(data map[string]interface{}, commandID, signature string, ts int64) (bool, error) {
	settingRepo := repositories.GetSystemSettingRepository()
	var panelKeys map[string]interface{}
	if err := settingRepo.GetJSON("panel_rsa_keys", &panelKeys); err != nil || panelKeys == nil {
		return false, fmt.Errorf("面板密钥对不存在")
	}
	publicKey, _ := panelKeys["panel_public_key"].(string)
	if publicKey == "" {
		return false, fmt.Errorf("面板公钥不存在")
	}
	canonical, err := canonicalAgentCommandJSON(data)
	if err != nil {
		return false, err
	}
	signed := append(canonical, []byte(fmt.Sprintf("|%s|%d", commandID, ts))...)
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, err
	}
	return cryptoutil.VerifySignature(signed, sigBytes, publicKey)
}
