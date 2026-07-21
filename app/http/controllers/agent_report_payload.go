package controllers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

const maxAgentReportDecompressedBytes = 4 * 1024 * 1024

func decodeAgentReportPayload(data interface{}) (interface{}, error) {
	wrapper, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	compressed, _ := wrapper["compressed"].(bool)
	if !compressed {
		return data, nil
	}

	compression, _ := wrapper["compression"].(string)
	encoding, _ := wrapper["encoding"].(string)
	payload, _ := wrapper["payload"].(string)
	if compression != "gzip" || encoding != "base64" || payload == "" {
		return nil, errors.New("压缩上报格式错误")
	}

	compressedBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}

	gr, err := gzip.NewReader(bytes.NewReader(compressedBytes))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	raw, err := io.ReadAll(io.LimitReader(gr, maxAgentReportDecompressedBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxAgentReportDecompressedBytes {
		return nil, errors.New("压缩上报解压后超过大小限制")
	}

	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}
