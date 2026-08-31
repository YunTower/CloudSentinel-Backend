package services

import (
	"goravel/app/facades"
)

// AgentCommandSender 统一的 Agent 命令下发入口：
// 构造 payload → 面板私钥签名 → 经 WS 通道发送（SendToAgent 内部按连接
// 加密状态自动分派明文/加密帧）。
// 此前 restart / update / update_config 三处各自重复"组装 map + 签名"，
// 且签名失败的处理策略不一致；收敛后签名失败统一返回错误，由调用方决定
// 拒绝下发的方式。
func SendSignedAgentCommand(serverID, command, commandID string, data map[string]interface{}) error {
	sig, ts, err := SignAgentCommand(data, commandID)
	if err != nil {
		return err
	}
	message := map[string]interface{}{
		"type":    "command",
		"command": command,
		"data":    data,
		"sig":     sig,
		"sig_ts":  ts,
	}
	if commandID != "" {
		message["command_id"] = commandID
	}
	if err := GetWebSocketService().SendMessage(serverID, message); err != nil {
		facades.Log().Errorf("发送 Agent 命令失败: server_id=%s command=%s error=%v", serverID, command, err)
		return err
	}
	return nil
}
