package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-rpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0x3bef5c // 魔数

/*
  1. Option 固定使用json来序列化，至于body和header如何序列化由codec.Type决定
  2. | option | Header1 | body1 | Header2 | body2 | ...
*/

type Option struct {
	MagicNumber    int           // 表明这是一个rpc请求
	CodecType      codec.Type    // client使用何种方式来对body进行编码
	ConnectTimeout time.Duration // 连接超时
	HandlerTimeout time.Duration // 处理超时
}

var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.JsonType,
	ConnectTimeout: 5 * time.Second,
	HandlerTimeout: 5 * time.Second,
}

type Server struct {
	serviceMap sync.Map // 并发安全，注册service
}

func (server *Server) Register(service any) error {
	s := NewService(service)
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc: service already defined: " + s.name)
	}
	return nil
}

//func Register(service any) error {
//	return DefaultServer.register(service)
//}

// 寻找service中的方法并返回用于调用,
func (server *Server) findService(serviceMethod string) (service *Service, method *methodType, err error) {
	// 校验 serviceMethod Foo.sum 并切割获取service和method名称
	index := strings.LastIndex(serviceMethod, ".")
	if index < 0 {
		err = errors.New("rpc server: invalid service method")
		return
	}
	serviceName, methodName := serviceMethod[:index], serviceMethod[index+1:]
	// 获取service
	svc, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	// 查找service中的method
	service = svc.(*Service)
	method = service.Method[methodName]
	if method == nil {
		err = errors.New("rpc server: can't find method " + methodName)
		return
	}
	return
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

func (server *Server) Accept(listener net.Listener) {
	// for循环接受请求
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("rpc server -> accept error: ", err)
		}
		go server.ServerConn(conn) // 交给服务端实例处理
	}
}

func (server *Server) ServerConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()
	var option Option // 解码并设置到Option中
	if err := json.NewDecoder(conn).Decode(&option); err != nil {
		log.Println("rpc server -> decode error: ", err)
		return
	}
	if option.MagicNumber != MagicNumber {
		log.Printf("rpc server -> invalid magic number: %d\n", option.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[option.CodecType] // 根据option传入的类型获取解析方法
	if f == nil {
		log.Printf("rpc server -> no impl by codecType: %s\n", option.CodecType)
		return
	}
	server.serverCodec(f(conn), option.HandlerTimeout)
}

var invalidRequest = struct{}{}

// 一次连接可能有多个请求，所以需要for循环等待，直到错误发生退出
func (server *Server) serverCodec(f codec.Codec, timeout time.Duration) {
	sending := new(sync.Mutex) // 保证response有序
	wg := new(sync.WaitGroup)
	// 允许一次连接中，接收多个请求，即多个header和body
	// 请求可以并发处理，但是响应必须是逐个发送
	for {
		req, err := server.readRequest(f) // 解析请求
		if err != nil {                   // 如果解析失败，需要返回response
			if req == nil { // 表示header解析失败，那么跳出这次请求
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(f, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go server.handleRequest(f, req, sending, wg, timeout) // 协程处理
	}
	wg.Wait()
	_ = f.Close()
}

type request struct {
	h           *codec.Header // 请求的header
	argv, reply reflect.Value // 请求的参数和响应参数
	mType       *methodType
	service     *Service
}

// 解析header
func (server *Server) readRequestHeader(c codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := c.ReadHeader(&h); err != nil {
		if err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println("rpc server readRequestHeader: ", err)
		}
		return nil, err
	}
	return &h, nil
}

// 解析request, 返回nil表示解析header失败
func (server *Server) readRequest(c codec.Codec) (*request, error) {
	header, err := server.readRequestHeader(c)
	if err != nil {
		return nil, err
	}
	req := &request{
		h: header,
	}
	req.service, req.mType, err = server.findService(header.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mType.NewArgv()
	req.reply = req.mType.NewReply()
	// 保证入参的argv是指针类型，因为ReadBody方法需要传入指针类型
	argvI := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Pointer {
		argvI = req.argv.Addr().Interface()
	}
	if err = c.ReadBody(argvI); err != nil {
		log.Println("rpc server -> read argv ")
	}
	return req, nil
}

// 输出服务端响应
func (server *Server) sendResponse(c codec.Codec, h *codec.Header, r any, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := c.Write(h, r); err != nil { // 加锁依次输出响应
		log.Println("rpc server: write response error: ", err)
	}
}

func (server *Server) handleRequest(f codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{})
	sent := make(chan struct{})
	log.Println("rpc server, handler request: ", req.h, req.argv)
	go func() {
		err := req.service.Call(req.mType, req.argv, req.reply) // 真正调用service中的method方法
		time.Sleep(10 * time.Second)
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			server.sendResponse(f, req.h, invalidRequest, sending)
			return
		}
		server.sendResponse(f, req.h, req.reply.Interface(), sending)
		sent <- struct{}{}
	}()
	if timeout == 0 {
		<-called
		<-sent
		return
	}
	select {
	case <-time.After(timeout): // 如果先于called执行，说明called超时，那么直接调用响应返回
		req.h.Error = fmt.Sprintf("rpc server: request handler timeout: expect within %s", timeout)
		server.sendResponse(f, req.h, invalidRequest, sending)
	case <-called: // 如果先于time.After执行，说明called没有超时，那么等待sent执行完毕
		<-sent
	}
}
