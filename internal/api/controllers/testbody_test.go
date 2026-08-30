package controllers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// jsonBody 将任意值序列化为 JSON 请求体
func jsonBody(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return bytes.NewBuffer(b)
}

// strBody 构造原始字符串请求体
func strBody(s string) *bytes.Buffer {
	return bytes.NewBufferString(s)
}

// jsonUnmarshalBody 反序列化 httptest 响应体
func jsonUnmarshalBody(w *httptest.ResponseRecorder, v interface{}) error {
	return json.Unmarshal(w.Body.Bytes(), v)
}
