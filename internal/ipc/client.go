package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	http      *http.Client
	transport *http.Transport
	token     string
}

func NewClient(credentials Credentials) *Client {
	return NewClientWithDialer(credentials.Token, func(ctx context.Context) (net.Conn, error) {
		return dialPipe(ctx, credentials.Pipe)
	})
}

func NewClientWithDialer(token string, dial func(context.Context) (net.Conn, error)) *Client {
	transport := &http.Transport{
		DialContext:           func(ctx context.Context, _, _ string) (net.Conn, error) { return dial(ctx) },
		MaxConnsPerHost:       4,
		MaxIdleConnsPerHost:   2,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &Client{http: &http.Client{Transport: transport, Timeout: 20 * time.Second}, transport: transport, token: token}
}

func (c *Client) Close() { c.transport.CloseIdleConnections() }
func (c *Client) Status(ctx context.Context) (State, error) {
	var v State
	err := c.call(ctx, "GET", "/v1/status", nil, &v)
	return v, err
}
func (c *Client) Snapshot(ctx context.Context) (Traffic, error) {
	var v Traffic
	err := c.call(ctx, "GET", "/v1/traffic", nil, &v)
	return v, err
}
func (c *Client) Start(ctx context.Context, data json.RawMessage) error {
	return c.call(ctx, "POST", "/v1/start", data, nil)
}
func (c *Client) Stop(ctx context.Context) error {
	return c.call(ctx, "POST", "/v1/stop", struct{}{}, nil)
}
func (c *Client) SetMode(ctx context.Context, mode string) error {
	return c.call(ctx, "POST", "/v1/mode", ModeRequest{mode}, nil)
}
func (c *Client) SelectNode(ctx context.Context, group, node string) error {
	return c.call(ctx, "POST", "/v1/select", SelectRequest{group, node}, nil)
}

func (c *Client) call(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return errors.New("请求格式无效")
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://ferry"+path, body)
	if err != nil {
		return errors.New("无法创建内核请求")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("无法连接内核，请重新启动")
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodySize+1))
	if err != nil || len(data) > MaxBodySize {
		return errors.New("内核响应无效")
	}
	if resp.StatusCode != 200 {
		return errors.New("内核操作失败，请检查配置或端口占用")
	}
	if output != nil && json.Unmarshal(data, output) != nil {
		return errors.New("内核响应格式无效")
	}
	return nil
}
