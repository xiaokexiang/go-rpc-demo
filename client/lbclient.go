package client

import (
	"context"
	"go-rpc/server"
	"reflect"
	"sync"
)

// LoadBalanceClient 支持负载均衡的客户端
type LoadBalanceClient struct {
	d       Discovery
	mode    SelectMode
	opt     *server.Option
	mu      sync.Mutex
	clients map[string]*Client
}

func NewLoadBalanceClient(d Discovery, mode SelectMode, opt *server.Option) *LoadBalanceClient {
	return &LoadBalanceClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

func (c *LoadBalanceClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, client := range c.clients { // 依次关闭客户端
		_ = client.Close()
		delete(c.clients, key)
	}
	return nil
}

// 判断client是否可用及生成客户端缓存
func (c *LoadBalanceClient) dial(addr string) (*Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	client, ok := c.clients[addr]
	if ok && !client.IsAvailable() { // 如果client存在但是已经close了就移除
		_ = client.Close()
		delete(c.clients, addr)
		client = nil
	}
	if client == nil {
		var err error
		client, err = Dial("tcp", addr, c.opt) // 重新生成客户端
		if err != nil {
			return nil, err
		}
		c.clients[addr] = client
	}
	return client, nil
}

// 负载均衡client内部还是调用了client的sync方法
func (c *LoadBalanceClient) call(addr string, ctx context.Context, serviceMethod string, args, reply any) error {
	client, err := c.dial(addr)
	if err != nil {
		return err
	}
	return client.Sync(ctx, serviceMethod, args, reply)
}

// Call 根据负载均衡模式获取一个地址
func (c *LoadBalanceClient) Call(ctx context.Context, serviceMethod string, args, reply any) error {
	addr, err := c.d.Get(c.mode)
	if err != nil {
		return err
	}
	return c.call(addr, ctx, serviceMethod, args, reply)
}

func (c *LoadBalanceClient) Broadcast(ctx context.Context, serviceMethod string, arg, reply any) error {
	servers, err := c.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error              // 用来处理并发请求中有一个失败就返回其中一个错误
	hasReply := reply == nil // 多个调用，有一个成功了有值就返回结果
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, addr := range servers {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			var r any
			if reply != nil {
				r = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface() // 创建类型一致的新实例
			}
			err := c.call(addr, ctx, serviceMethod, arg, r)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				cancel()
			}
			if err == nil && !hasReply {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(r).Elem()) // 将call的结果r赋值给reply
				hasReply = true
			}
			mu.Unlock()
		}(addr)
		wg.Wait()
	}
	return e
}
