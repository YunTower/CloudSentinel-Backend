package controllers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

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

	raw, err := io.ReadAll(gr)
	if err != nil {
		return nil, err
	}

	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}
