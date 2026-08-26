package push

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewIndexNowClient(t *testing.T) {
	client := NewIndexNowClient("test-api-key")
	assert.NotNil(t, client)
	assert.Equal(t, "test-api-key", client.apiKey)
	assert.NotNil(t, client.httpCli)
}

func TestIndexNowClient_Push_EmptyAPIKey(t *testing.T) {
	client := NewIndexNowClient("")
	_, err := client.Push([]string{"https://example.com"}, "example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API key is not configured")
}

func TestIndexNowClient_Push_NoURLs(t *testing.T) {
	client := NewIndexNowClient("test-key")
	result, err := client.Push([]string{}, "example.com")
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Message, "no URLs to push")
}
