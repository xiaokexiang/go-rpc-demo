package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-rpc/codec"
	"go-rpc/server"
	"log"
	"net"
	"sync"
)

// Call 表明一次客户端RPC请求携带的信息
// func (t *T) MethodName(argType T1, replyType *T2) error
type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          any
	Reply         any
	Error         error
	Done          chan *Call // 调用结束的时候通过管道通知对方
}

func (c *Call) done() {
	c.Done <- c
}

type Client struct {
	c        codec.Codec    // 客户端编解码逻辑
	option   *server.Option // 预请求传递的option
	sending  sync.Mutex     // 保证消息有序发送
	header   codec.Header
	mu       sync.Mutex
	seq      uint64
	pending  map[uint64]*Call // 存储未完成的请求
	closing  bool             // 用户是否关闭请求
	shutdown bool             // 服务端是否宕机
}

var ErrorShutDown = errors.New("connection is shutdown")

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrorShutDown
	}
	c.closing = true
	return c.c.Close()
}

// 判断服务是否中断且关闭
func (c *Client) available() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.shutdown && !c.closing
}

func (c *Client) registryCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.shutdown {
		return 0, ErrorShutDown
	}
	call.Seq = c.seq // 请求的序号由客户端决定
	c.pending[call.Seq] = call
	c.seq++
	return call.Seq, nil
}

// 将call从pending队列移除,不会发起请求
func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

// 服务端或客户端发生错误时调用
func (c *Client) terminateCall(err error) {
	c.sending.Lock()
	defer c.sending.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done() // 发送通知到所有的call
	}
}

/*
client 发送 call 到server端, 通过seq来绑定关联
1. call 不存在，可能是请求没有发送完整，或者因为其他原因被取消，但是服务端仍旧处理了。
2. call 存在，但服务端处理出错，即 h.Error 不为空。
3. call 存在，服务端处理正常，那么需要从 body 中读取 Reply 的值。
*/
func (c *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = c.c.ReadHeader(&h); err != nil {
			break
		}
		call := c.removeCall(h.Seq)
		switch {
		case call == nil:
			err = c.c.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = c.c.ReadBody(nil)
			call.done()
		default:
			err := c.c.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	c.terminateCall(err) // 报错终止
}

func (c *Client) send(call *Call) {
	c.sending.Lock()
	defer c.sending.Unlock()

	seq, err := c.registryCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Seq = seq
	c.header.Error = ""

	if err := c.c.Write(&c.header, call.Args); err != nil {
		call := c.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func NewClient(conn net.Conn, opt *server.Option) (*Client, error) {
	f := codec.FuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invaild code type %s", opt.CodecType)
		log.Println("RPC [Client] Code Error: ", err)
		return nil, err
	}
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("RPC [Client] options error: ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(f codec.Codec, option *server.Option) *Client {
	client := &Client{
		seq:     1,
		c:       f,
		option:  option,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

func parseOptions(opts ...*server.Option) (*server.Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return server.DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNum = server.MagicNum
	if opt.CodecType == "" {
		opt.CodecType = server.DefaultOption.CodecType
	}
	return opt, nil
}

func Dial(network, address string, opts ...*server.Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()
	return NewClient(conn, opt)
}

func (c *Client) SendAsync(serviceMethod string, args, reply any, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	c.send(call)
	return call
}

func (c *Client) SendSync(serviceMethod string, args, reply any) error {
	call := <-c.SendAsync(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
