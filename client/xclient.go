package client

import (
	"context"
	"fmt"
	"go-rpc/server"
	"io"
	"reflect"
	"strings"
	"sync"
)

// XClient 支持负载均衡的客户端
type XClient struct {
	d       server.Discovery
	mode    server.SelectMode
	opt     *server.Option
	mu      sync.Mutex
	clients map[string]*Client
}

var _ io.Closer = (*XClient)(nil) // 保证XClient实现close接口

func NewXClient(d server.Discovery, mode server.SelectMode, opt *server.Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

func (c *XClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, client := range c.clients {
		_ = client.Close()
		delete(c.clients, key)
	}
	return nil
}

func XDial(address string, opts ...*server.Option) (client *Client, err error) {
	parts := strings.Split(address, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client err: wrong format '%s', expect protocol@addr", address)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		// tcp, unix or other transport protocol
		return Dial(protocol, addr, opts...)
	}
}

// 从缓存获取或在重新生成客户端
func (c *XClient) dial(addr string) (*Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	client, ok := c.clients[addr]
	if ok && !client.available() { // client存在但是不可达
		_ = client.Close()
		delete(c.clients, addr)
		client = nil
	}
	if client == nil { // 重新生成客户端到缓存中
		var err error
		client, err = XDial(addr, c.opt)
		if err != nil {
			return nil, err
		}
		c.clients[addr] = client
	}
	return client, nil
}

func (c *XClient) call(addr string, ctx context.Context, serviceMethod string, args, reply any) error {
	// 获取客户端
	client, err := c.dial(addr)
	if err != nil {
		return err
	}
	return client.SendSync(ctx, serviceMethod, args, reply)
}

// Call 基于负载均衡策略进行请求
func (c *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	// 基于Discovery获取客户端请求的服务端地址
	addr, err := c.d.Get(c.mode)
	if err != nil {
		return err
	}
	return c.call(addr, ctx, serviceMethod, args, reply)
}

// Broadcast 广播执行, 客户端获取绑定的服务端列表并并发执行,只要其中一个执行完毕就返回结果
func (c *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	servers, err := c.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error
	replyDone := reply == nil // 如果reply为nil则不需要设置返回值
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, addr := range servers {
		wg.Add(1)
		go func(server string) {
			defer wg.Done()
			var r any
			if reply != nil {
				r = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := c.call(server, ctx, serviceMethod, args, r)
			mu.Lock() // 多个goroutine对server操作,所以加锁同步保证
			if err != nil && e == nil {
				e = err
				cancel()
			}
			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(r).Elem()) // 将其中一个goroutine的返回值设置到reply中并标记为执行完毕
				replyDone = true
			}
			mu.Unlock()
		}(addr)
	}
	wg.Wait()
	return e
}
