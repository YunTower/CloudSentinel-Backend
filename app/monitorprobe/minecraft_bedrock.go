package monitorprobe

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var rakNetMagic = []byte{0x00, 0xff, 0xff, 0x00, 0xfe, 0xfe, 0xfe, 0xfe, 0xfd, 0xfd, 0xfd, 0xfd, 0x12, 0x34, 0x56, 0x78}

func checkMinecraftBedrock(ctx context.Context, request Request) Result {
	port := request.Port
	if port <= 0 {
		port = 19132
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp", net.JoinHostPort(request.Target, strconv.Itoa(port)))
	if err != nil {
		return networkFailed(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	ping := bytes.NewBuffer(make([]byte, 0, 33))
	ping.WriteByte(0x01)
	timestamp := time.Now().UnixMilli()
	_ = binary.Write(ping, binary.BigEndian, timestamp)
	ping.Write(rakNetMagic)
	var guid uint64
	if err := binary.Read(rand.Reader, binary.BigEndian, &guid); err != nil {
		return failed("probe_unavailable", err)
	}
	_ = binary.Write(ping, binary.BigEndian, guid)
	if _, err := conn.Write(ping.Bytes()); err != nil {
		return networkFailed(err)
	}

	response := make([]byte, 2048)
	n, err := conn.Read(response)
	if err != nil {
		return networkFailed(err)
	}
	response = response[:n]
	if len(response) < 35 || response[0] != 0x1c || !bytes.Equal(response[17:33], rakNetMagic) {
		return failed("protocol_error", fmt.Errorf("Bedrock Unconnected Pong 无效"))
	}
	serverIDLength := int(binary.BigEndian.Uint16(response[33:35]))
	if serverIDLength <= 0 || serverIDLength > len(response)-35 {
		return failed("protocol_error", fmt.Errorf("Bedrock Server ID 长度无效"))
	}
	fields := strings.Split(string(response[35:35+serverIDLength]), ";")
	if len(fields) < 6 || fields[0] != "MCPE" {
		return failed("protocol_error", fmt.Errorf("Bedrock Server ID 字段无效"))
	}
	protocol, errProtocol := strconv.Atoi(fields[2])
	online, errOnline := strconv.Atoi(fields[4])
	maxPlayers, errMax := strconv.Atoi(fields[5])
	if errProtocol != nil || errOnline != nil || errMax != nil || online < 0 || maxPlayers < 0 {
		return failed("protocol_error", fmt.Errorf("Bedrock 状态数字字段无效"))
	}
	metadata := map[string]any{
		"kind":             "minecraft",
		"edition":          "bedrock",
		"version_name":     fields[3],
		"protocol_version": protocol,
		"motd":             strings.TrimSpace(fields[1]),
		"players_online":   online,
		"players_max":      maxPlayers,
	}
	if len(fields) > 7 && strings.TrimSpace(fields[7]) != "" {
		metadata["sub_motd"] = strings.TrimSpace(fields[7])
	}
	if len(fields) > 8 && strings.TrimSpace(fields[8]) != "" {
		metadata["game_mode"] = strings.TrimSpace(fields[8])
	}
	return Result{Status: StatusUp, Metadata: metadata}
}
