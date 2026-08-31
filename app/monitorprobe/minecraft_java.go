package monitorprobe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"goravel/app/utils/security"
)

const maxMinecraftStatusPacket = 1 << 20

// lookupMinecraftSRV 保留为变量，便于在不访问公网 DNS 的情况下覆盖完整探测流程。
var lookupMinecraftSRV = net.DefaultResolver.LookupSRV

func checkMinecraftJava(ctx context.Context, request Request) Result {
	port := request.Port
	if port <= 0 {
		port = 25565
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialTarget, dialPort, usedSRV := resolveMinecraftJavaEndpoint(ctx, request.Target, port)
	// SSRF 防护：校验目标主机是否属于内网/保留地址。
	if err := security.ValidateHostForOutboundRequest(dialTarget, 2*time.Second, request.AllowPrivate); err != nil {
		return failed("ssrf_blocked", err)
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(dialTarget, strconv.Itoa(dialPort)))
	if err != nil {
		return networkFailed(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	handshake := bytes.NewBuffer(nil)
	handshake.Write(encodeVarInt(-1))
	handshake.Write(encodeVarInt(int32(len(request.Target))))
	handshake.WriteString(request.Target)
	_ = handshake.WriteByte(byte(port >> 8))
	_ = handshake.WriteByte(byte(port))
	handshake.Write(encodeVarInt(1))
	if err := writeMinecraftPacket(conn, 0, handshake.Bytes()); err != nil {
		return networkFailed(err)
	}
	if err := writeMinecraftPacket(conn, 0, nil); err != nil {
		return networkFailed(err)
	}

	reader := bufio.NewReader(conn)
	packetLength, err := readVarInt(reader)
	if err != nil {
		return networkFailed(err)
	}
	if packetLength <= 0 || packetLength > maxMinecraftStatusPacket {
		return failed("protocol_error", fmt.Errorf("Java 状态包长度无效"))
	}
	packet := make([]byte, packetLength)
	if _, err := io.ReadFull(reader, packet); err != nil {
		return networkFailed(fmt.Errorf("读取 Java 状态包失败: %w", err))
	}
	packetReader := bufio.NewReader(bytes.NewReader(packet))
	packetID, err := readVarInt(packetReader)
	if err != nil || packetID != 0 {
		return failed("protocol_error", fmt.Errorf("Java 状态包 ID 无效"))
	}
	jsonLength, err := readVarInt(packetReader)
	if err != nil || jsonLength < 0 || jsonLength > maxMinecraftStatusPacket || jsonLength > packetReader.Buffered() {
		return failed("protocol_error", fmt.Errorf("Java 状态 JSON 长度无效"))
	}
	raw := make([]byte, jsonLength)
	if _, err := io.ReadFull(packetReader, raw); err != nil {
		return failed("protocol_error", fmt.Errorf("读取 Java 状态 JSON 失败: %w", err))
	}
	var payload struct {
		Version struct {
			Name     string `json:"name"`
			Protocol int    `json:"protocol"`
		} `json:"version"`
		Players struct {
			Max    int `json:"max"`
			Online int `json:"online"`
		} `json:"players"`
		Description json.RawMessage `json:"description"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload.Description) == 0 {
		return failed("protocol_error", fmt.Errorf("Java 状态 JSON 无效"))
	}
	metadata := map[string]any{
		"kind":             "minecraft",
		"edition":          "java",
		"version_name":     payload.Version.Name,
		"protocol_version": payload.Version.Protocol,
		"motd":             flattenMinecraftDescription(payload.Description),
		"players_online":   payload.Players.Online,
		"players_max":      payload.Players.Max,
	}
	if usedSRV {
		metadata["srv_target"] = dialTarget
		metadata["srv_port"] = dialPort
	}
	return Result{Status: StatusUp, Metadata: metadata}
}

// resolveMinecraftJavaEndpoint 遵循 Java 客户端规则：默认端口时可由 _minecraft._tcp SRV
// 记录指定实际连接目标。未配置或无法解析 SRV 时回退至用户填写的主机和端口。
func resolveMinecraftJavaEndpoint(ctx context.Context, target string, port int) (string, int, bool) {
	if port != 25565 || net.ParseIP(target) != nil {
		return target, port, false
	}
	_, records, err := lookupMinecraftSRV(ctx, "minecraft", "tcp", target)
	if err != nil || len(records) == 0 {
		return target, port, false
	}
	for _, record := range records {
		resolvedTarget := strings.TrimSuffix(strings.TrimSpace(record.Target), ".")
		if resolvedTarget != "" && resolvedTarget != "." && record.Port > 0 {
			return resolvedTarget, int(record.Port), true
		}
	}
	return target, port, false
}

func writeMinecraftPacket(w io.Writer, packetID int32, payload []byte) error {
	body := append(encodeVarInt(packetID), payload...)
	packet := append(encodeVarInt(int32(len(body))), body...)
	_, err := w.Write(packet)
	return err
}

func encodeVarInt(value int32) []byte {
	unsigned := uint32(value)
	encoded := make([]byte, 0, 5)
	for {
		current := byte(unsigned & 0x7f)
		unsigned >>= 7
		if unsigned != 0 {
			current |= 0x80
		}
		encoded = append(encoded, current)
		if unsigned == 0 {
			return encoded
		}
	}
}

func readVarInt(reader io.ByteReader) (int, error) {
	var value uint32
	for position := 0; position < 5; position++ {
		current, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= uint32(current&0x7f) << (7 * position)
		if current&0x80 == 0 {
			return int(int32(value)), nil
		}
	}
	return 0, fmt.Errorf("VarInt 过长")
}

func flattenMinecraftDescription(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text)
	}
	var component struct {
		Text  string            `json:"text"`
		Extra []json.RawMessage `json:"extra"`
	}
	if json.Unmarshal(raw, &component) != nil {
		return ""
	}
	parts := []string{component.Text}
	for _, extra := range component.Extra {
		parts = append(parts, flattenMinecraftDescription(extra))
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}
