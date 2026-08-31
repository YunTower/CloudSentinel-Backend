package monitorprobe

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestCheckMinecraftJava默认端口使用SRV记录(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatal(err)
	}

	originalLookupSRV := lookupMinecraftSRV
	lookupMinecraftSRV = func(_ context.Context, service, proto, name string) (string, []*net.SRV, error) {
		if service != "minecraft" || proto != "tcp" || name != "play.example.com" {
			t.Fatalf("SRV 查询参数 = %s %s %s", service, proto, name)
		}
		return "", []*net.SRV{{Target: "127.0.0.1.", Port: uint16(port)}}, nil
	}
	t.Cleanup(func() { lookupMinecraftSRV = originalLookupSRV })

	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		if err := discardMinecraftTestPacket(reader); err != nil {
			serverErr <- err
			return
		}
		if err := discardMinecraftTestPacket(reader); err != nil {
			serverErr <- err
			return
		}
		statusJSON := []byte(`{"version":{"name":"1.21.8","protocol":772},"players":{"max":10,"online":1},"description":"SRV Java"}`)
		body := append([]byte{0}, append(encodeMinecraftTestVarInt(len(statusJSON)), statusJSON...)...)
		_, err = conn.Write(append(encodeMinecraftTestVarInt(len(body)), body...))
		serverErr <- err
	}()

	result := Check(context.Background(), Request{
		Type: TypeMinecraftJava, Target: "play.example.com", Port: 25565, Timeout: 2 * time.Second, AllowPrivate: true,
	})
	if result.Status != StatusUp || result.Error != nil {
		t.Fatalf("探测结果 = %#v", result)
	}
	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}

func encodeMinecraftTestVarInt(value int) []byte {
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

func discardMinecraftTestPacket(reader *bufio.Reader) error {
	length := 0
	for shift := 0; shift < 35; shift += 7 {
		current, err := reader.ReadByte()
		if err != nil {
			return err
		}
		length |= int(current&0x7f) << shift
		if current&0x80 == 0 {
			_, err = io.CopyN(io.Discard, reader, int64(length))
			return err
		}
	}
	return fmt.Errorf("测试请求 VarInt 过长")
}
