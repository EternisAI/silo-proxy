package client

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/EternisAI/silo-proxy/proto"
)

type RequestHandler struct {
	httpClient *http.Client
	localURL   string
}

func NewRequestHandler(localURL string) *RequestHandler {
	return &RequestHandler{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		localURL: localURL,
	}
}

func (rh *RequestHandler) HandleRequest(msg *proto.ProxyMessage) (*proto.ProxyMessage, error) {
	method := msg.Metadata["method"]
	path := msg.Metadata["path"]
	query := msg.Metadata["query"]

	url := rh.localURL + path
	if query != "" {
		url += "?" + query
	}

	slog.Info("Forwarding request to local service",
		"message_id", msg.Id,
		"method", method,
		"url", url)

	req, err := http.NewRequest(method, url, bytes.NewReader(msg.Payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range msg.Metadata {
		if strings.HasPrefix(key, "header_") {
			headerName := strings.TrimPrefix(key, "header_")
			req.Header.Set(headerName, value)
		}
	}

	if contentType, ok := msg.Metadata["content_type"]; ok && contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := rh.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	slog.Info("Received response from local service",
		"message_id", msg.Id,
		"status_code", resp.StatusCode,
		"content_length", len(body))

	responseMsg := &proto.ProxyMessage{
		Id:      msg.Id,
		Type:    proto.MessageType_RESPONSE,
		Payload: body,
		Metadata: map[string]string{
			"status_code":  strconv.Itoa(resp.StatusCode),
			"content_type": resp.Header.Get("Content-Type"),
		},
	}

	return responseMsg, nil
}
