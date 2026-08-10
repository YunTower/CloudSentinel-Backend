package monitorprobe_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goravel/app/monitorprobe"
)

func testVarInt(value int) []byte {
	result := make([]byte, 0, 5)
	for {
		current := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			current |= 0x80
		}
		result = append(result, current)
		if value == 0 {
			return result
		}
	}
}

func discardTestMinecraftPacket(reader *bufio.Reader) error {
	length := 0
	shift := 0
	for i := 0; i < 5; i++ {
		current, err := reader.ReadByte()
		if err != nil {
			return err
		}
		length |= int(current&0x7f) << shift
		if current&0x80 == 0 {
			_, err = io.CopyN(io.Discard, reader, int64(length))
			return err
		}
		shift += 7
	}
	return fmt.Errorf("测试请求 VarInt 过长")
}

func TestCheckChatCompletions发送最小真实请求(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("请求方法 = %s，期望 POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","model":"model-a","choices":[{"index":0,"finish_reason":"length","message":{"role":"assistant","content":"p"}}]}`))
	}))
	defer server.Close()

	result := monitorprobe.Check(context.Background(), monitorprobe.Request{
		Type:     monitorprobe.TypeAIModel,
		Target:   server.URL,
		Timeout:  2 * time.Second,
		AIFormat: monitorprobe.AIFormatChatCompletions,
		AIModel:  "model-a",
		AIAPIKey: "test-key",
	})

	if result.Status != monitorprobe.StatusUp || result.Error != nil {
		t.Fatalf("探测结果 = %#v, error=%v", result, result.Error)
	}
	if requestBody["model"] != "model-a" || requestBody["max_tokens"] != float64(1) || requestBody["stream"] != false {
		t.Fatalf("请求体不是最小探测请求: %#v", requestBody)
	}
	messages, ok := requestBody["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %#v", requestBody["messages"])
	}
}

func TestCheckAnthropicMessages接受最小输出截断(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "anthropic-key" {
			t.Fatalf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Fatalf("anthropic-version = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg-test","type":"message","model":"claude-test","content":[{"type":"text","text":"p"}],"stop_reason":"max_tokens"}`))
	}))
	defer server.Close()

	result := monitorprobe.Check(context.Background(), monitorprobe.Request{
		Type:     monitorprobe.TypeAIModel,
		Target:   server.URL,
		Timeout:  2 * time.Second,
		AIFormat: monitorprobe.AIFormatAnthropicMessages,
		AIModel:  "claude-test",
		AIAPIKey: "anthropic-key",
	})

	if result.Status != monitorprobe.StatusUp || result.Error != nil {
		t.Fatalf("探测结果 = %#v", result)
	}
	if requestBody["model"] != "claude-test" || requestBody["max_tokens"] != float64(1) {
		t.Fatalf("请求体不是最小探测请求: %#v", requestBody)
	}
}

func TestCheckResponses接受最大输出限制导致的不完整响应(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp-test","object":"response","model":"model-r","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[]}`))
	}))
	defer server.Close()

	result := monitorprobe.Check(context.Background(), monitorprobe.Request{
		Type:     monitorprobe.TypeAIModel,
		Target:   server.URL,
		Timeout:  2 * time.Second,
		AIFormat: monitorprobe.AIFormatResponses,
		AIModel:  "model-r",
		AIAPIKey: "response-key",
	})

	if result.Status != monitorprobe.StatusUp || result.Error != nil {
		t.Fatalf("探测结果 = %#v", result)
	}
	if requestBody["input"] != "ping" || requestBody["max_output_tokens"] != float64(1) || requestBody["stream"] != false {
		t.Fatalf("请求体不是最小探测请求: %#v", requestBody)
	}
}

func TestCheckMinecraftJava解析状态响应(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		if err := discardTestMinecraftPacket(reader); err != nil {
			serverErr <- fmt.Errorf("未收到握手: %w", err)
			return
		}
		if err := discardTestMinecraftPacket(reader); err != nil {
			serverErr <- fmt.Errorf("未收到状态请求: %w", err)
			return
		}

		statusJSON := []byte(`{"version":{"name":"1.21.8","protocol":772},"players":{"max":100,"online":12},"description":{"text":"云塔生存服"}}`)
		packet := bytes.NewBuffer(nil)
		packet.WriteByte(0)
		packet.Write(testVarInt(len(statusJSON)))
		packet.Write(statusJSON)
		response := append(testVarInt(packet.Len()), packet.Bytes()...)
		if _, err := io.Copy(conn, bytes.NewReader(response)); err != nil {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	var port int
	_, _ = fmt.Sscanf(portText, "%d", &port)
	result := monitorprobe.Check(context.Background(), monitorprobe.Request{
		Type:    monitorprobe.TypeMinecraftJava,
		Target:  host,
		Port:    port,
		Timeout: 2 * time.Second,
	})

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	if result.Status != monitorprobe.StatusUp || result.Error != nil {
		t.Fatalf("探测结果 = %#v", result)
	}
	if result.Metadata["version_name"] != "1.21.8" || result.Metadata["players_online"] != 12 || result.Metadata["motd"] != "云塔生存服" {
		t.Fatalf("元数据 = %#v", result.Metadata)
	}
}

func TestCheckMinecraftBedrock解析UnconnectedPong(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	serverErr := make(chan error, 1)
	go func() {
		buffer := make([]byte, 2048)
		n, addr, err := server.ReadFrom(buffer)
		if err != nil {
			serverErr <- err
			return
		}
		if n < 33 || buffer[0] != 0x01 {
			serverErr <- fmt.Errorf("无效 Unconnected Ping")
			return
		}
		serverID := []byte("MCPE;云塔基岩服;800;1.21.80;8;50;123456;Adventure;Survival;1;19132;19133;")
		response := bytes.NewBuffer(nil)
		response.WriteByte(0x1c)
		response.Write(buffer[1:9])
		_ = binary.Write(response, binary.BigEndian, uint64(123456))
		response.Write(buffer[9:25])
		_ = binary.Write(response, binary.BigEndian, uint16(len(serverID)))
		response.Write(serverID)
		_, err = server.WriteTo(response.Bytes(), addr)
		serverErr <- err
	}()

	host, portText, _ := net.SplitHostPort(server.LocalAddr().String())
	var port int
	_, _ = fmt.Sscanf(portText, "%d", &port)
	result := monitorprobe.Check(context.Background(), monitorprobe.Request{
		Type:    monitorprobe.TypeMinecraftBedrock,
		Target:  host,
		Port:    port,
		Timeout: 2 * time.Second,
	})

	if result.Status != monitorprobe.StatusUp || result.Error != nil {
		t.Fatalf("探测结果 = %#v, error=%v", result, result.Error)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
	if result.Metadata["edition"] != "bedrock" || result.Metadata["players_online"] != 8 || result.Metadata["players_max"] != 50 || result.Metadata["motd"] != "云塔基岩服" {
		t.Fatalf("元数据 = %#v", result.Metadata)
	}
}

func TestCheckAI按HTTP状态分类且不跟随重定向(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		errorCode  string
	}{
		{name: "鉴权失败", statusCode: http.StatusUnauthorized, errorCode: "authentication_error"},
		{name: "请求受限", statusCode: http.StatusTooManyRequests, errorCode: "rate_limited"},
		{name: "服务异常", statusCode: http.StatusBadGateway, errorCode: "provider_error"},
		{name: "拒绝重定向", statusCode: http.StatusTemporaryRedirect, errorCode: "redirect_rejected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tc.statusCode >= 300 && tc.statusCode < 400 {
					w.Header().Set("Location", "https://example.invalid/secret")
				}
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(`{"error":{"message":"provider error"}}`))
			}))
			defer server.Close()

			result := monitorprobe.Check(context.Background(), monitorprobe.Request{
				Type:     monitorprobe.TypeAIModel,
				Target:   server.URL,
				Timeout:  2 * time.Second,
				AIFormat: monitorprobe.AIFormatChatCompletions,
				AIModel:  "model-a",
				AIAPIKey: "must-not-leak",
			})

			if result.Status != monitorprobe.StatusDown || result.ErrorCode != tc.errorCode {
				t.Fatalf("探测结果 = %#v", result)
			}
		})
	}
}

func TestCheckAI超时标记为缓慢(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(80 * time.Millisecond)
		_, _ = w.Write([]byte(`{"id":"late","object":"chat.completion","choices":[{}]}`))
	}))
	defer server.Close()

	result := monitorprobe.Check(context.Background(), monitorprobe.Request{
		Type:     monitorprobe.TypeAIModel,
		Target:   server.URL,
		Timeout:  20 * time.Millisecond,
		AIFormat: monitorprobe.AIFormatChatCompletions,
		AIModel:  "model-a",
	})

	if result.Status != monitorprobe.StatusSlow || result.ErrorCode != "timeout" {
		t.Fatalf("探测结果 = %#v", result)
	}
}
