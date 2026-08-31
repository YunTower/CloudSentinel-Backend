package panelkey

import (
	"fmt"
	"sync"

	"goravel/app/cryptoutil"
	"goravel/app/facades"
	"goravel/app/repositories"
)

// 面板 RSA 密钥对（用于 Agent 会话密钥交换与命令签名）的统一访问点。
//
// 此前 websocket/handler.go 与 services/agent_command_signature.go 各有一条
// "读取或生成" 路径且均无锁，首次认证与首条签名命令并发时可能各自生成
// 密钥对互相覆盖，导致已持有旧公钥的 Agent 持续验签失败。所有使用方必须
// 经由本包获取，生成与读取在同一临界区内完成。

var (
	mu     sync.Mutex
	loaded bool
	priv   string
	pub    string
)

// GetOrGenerate 返回面板私钥与公钥；不存在时生成并持久化到 system_settings。
func GetOrGenerate() (privateKey, publicKey string, err error) {
	mu.Lock()
	defer mu.Unlock()

	if loaded {
		return priv, pub, nil
	}

	settingRepo := repositories.GetSystemSettingRepository()
	var panelKeys map[string]interface{}
	if err := settingRepo.GetJSON("panel_rsa_keys", &panelKeys); err == nil && panelKeys != nil {
		if pk, ok := panelKeys["panel_private_key"].(string); ok && pk != "" {
			if p, ok := panelKeys["panel_public_key"].(string); ok && p != "" {
				priv, pub = pk, p
				loaded = true
				return priv, pub, nil
			}
		}
	}

	facades.Log().Info("Panel 密钥对不存在，正在生成...")
	privateKey, publicKey, err = cryptoutil.GenerateKeyPair()
	if err != nil {
		return "", "", fmt.Errorf("生成面板密钥对失败: %w", err)
	}

	panelKeys = map[string]interface{}{
		"panel_private_key": privateKey,
		"panel_public_key":  publicKey,
	}
	if err := settingRepo.SetJSON("panel_rsa_keys", panelKeys); err != nil {
		return "", "", fmt.Errorf("保存面板密钥对失败: %w", err)
	}

	facades.Log().Info("Panel 密钥对已生成并保存到 system_settings")
	priv, pub = privateKey, publicKey
	loaded = true
	return priv, pub, nil
}
