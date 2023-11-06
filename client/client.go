package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-rpc/codec"
	"go-rpc/server"
	"io"
	"log"
	"net"
	"sync"
)

// Call 用来承载rpc所需要的信息
type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          any
	Reply         any
	Error         error
	Done          chan *Call // 表示调用是否结束
}

// 将call实例传输到done通道
func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	c        codec.Codec
	option   *server.Option
	sending  sync.Mutex // 保证请求有序发送
	header   codec.Header
	mutex    sync.Mutex // 用来存储未处理完的请求
	seq      uint64
	pending  map[uint64]*Call
	closing  bool // 客户端是否主动关闭
	shutdown bool // 服务端是否主动关闭，表示有错误发生
}

var _ io.Closer = (*Client)(nil)

var ErrShutDown = errors.New("connection is shutdown")

// Close 关闭客户端
func (client *Client) Close() error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	if client.closing {
		return ErrShutDown
	}
	client.closing = true
	return client.c.Close()
}

// IsAvailable 判断客户端是否可用
func (client *Client) IsAvailable() bool {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	return !client.shutdown && !client.closing
}

// 注册任务
func (client *Client) registryCall(call *Call) (uint64, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	if client.closing || client.shutdown {
		return 0, ErrShutDown
	}

	call.Seq = client.seq
	client.pending[call.Seq] = call // 注册请求
	client.seq++
	return call.Seq, nil
}

// 从pending队列移除并返回call任务
func (client *Client) removeCall(seq uint64) *Call {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq) // 调用map删除
	return call
}

// 类似线程池关闭一样，将所有等待的任务都标记为完成并通知错误
func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mutex.Lock()
	defer client.mutex.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

func (client *Client) receive() {
	var err error
	for err == nil { // err不为nil就跳出循环
		var h codec.Header
		if err := client.c.ReadHeader(&h); err != nil {
			break
		}
		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			// 服务端处理了请求，但是客户端这边被取消了
			err = errors.New("client cancels request")
		case h.Error != "":
			// 服务端处理出错
			err = errors.New(h.Error)
			call.Error = err
			call.done()
		default:
			err = client.c.ReadBody(call.Reply) // 将输出写入到reply中
			if err != nil {
				call.Error = fmt.Errorf("reading body" + err.Error())
			}
			call.done()
		}
	}
	// 执行到此说明发生错误，停止客户端
	client.terminateCalls(err)
}

func NewClient(conn net.Conn, opt *server.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("unsupported codec type: %s", opt.CodecType)
		log.Println("rpc client, codec error: ", err)
		return nil, err
	}
	// 预先发送option
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client option error: ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(f codec.Codec, opt *server.Option) *Client {
	c := &Client{
		c:       f, // 客户端的解析方式
		seq:     1, // 默认从1开始
		option:  opt,
		pending: make(map[uint64]*Call),
	}
	go c.receive()
	return c
}

func parseOptions(opts ...*server.Option) (*server.Option, error) {
	if len(opts) > 1 {
		return nil, fmt.Errorf("number of options is more than 1")
	}
	if opts == nil || opts[0] == nil {
		return server.DefaultOption, nil
	}
	option := opts[0]
	option.MagicNumber = server.MagicNumber
	if option.CodecType == "" {
		option.CodecType = server.DefaultOption.CodecType
	}
	return option, nil
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

func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	seq, err := client.registryCall(call) // 注册call
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""
	if err := client.c.Write(&client.header, call.Args); err != nil { // 发送消息
		client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// Async 异步调用，返回call实例
func (client *Client) Async(serviceMethod string, args, reply any, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10) // 带缓存的通道
	} else if cap(done) == 0 {
		log.Panic("rpc client done channel is unbuffered")
	}
	c := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(c)
	return c
}

// Sync 同步调用，等待返回
func (client *Client) Sync(serviceMethod string, args, reply any) error {
	async := client.Async(serviceMethod, args, reply, make(chan *Call, 1))
	call := <-async.Done
	return call.Error
}
