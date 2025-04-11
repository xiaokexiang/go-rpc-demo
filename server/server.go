package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-rpc/codec"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Option struct {
	MagicNum       int           // mark this is a rpc request
	CodecType      codec.Type    // choose which codec to encode body
	ConnectTimeout time.Duration // 连接超时
	HandleTimeout  time.Duration // 处理超时, 0表示不限制
}

const MagicNum = 0x3bef5c

// DefaultOption
// | Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
// | <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|
// | Option | Header1 | Body1 | Header2 | Body2 | ...
var DefaultOption = &Option{
	MagicNum:       MagicNum,
	CodecType:      codec.GobType,
	ConnectTimeout: 10 * time.Second,
}

// Accept accepts connections on the listener and serves requests
func Accept(listen net.Listener) {
	DefaultServer.accept(listen)
}

// Accept Listener 定义了一个通用的网络监听器，用于接收流式协议（如 TCP、Unix Socket）的连接请求
// for循环等待socket建立, 处理交给子协程
func (s *Server) accept(listen net.Listener) {
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Println("RPC [Server] Accept error: ", err)
			return
		}
		go s.serverConn(conn)
	}
}

var DefaultServer = &Server{}

type Server struct {
	serviceMap sync.Map // 线程安全Map存储service
}

/*
1. 接受请求
2. 解析options请求,options用json序列化
*/
func (s *Server) serverConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()
	// 1. 解析option
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("RPC [ServerConn] Options error: ", err)
		return
	}
	if opt.MagicNum != MagicNum {
		log.Println("RPC [ServerConn] MagicNum error: ", opt.MagicNum)
		return
	}
	// 2.获取对应的编码器进行解码编码
	f := codec.FuncMap[opt.CodecType]
	if f == nil {
		log.Println("RPC [ServerConn] Invalid CodecType: ", opt.CodecType)
		return
	}
	s.serverCodec(f(conn), &opt)
}

/*
1. 解析请求 不符合请求 发送响应 2. 处理请求
*/
func (s *Server) serverCodec(c codec.Codec, opt *Option) {
	sending := new(sync.Mutex) // 保证响应依次发送
	wg := new(sync.WaitGroup)  // 保证所有请求被处理

	for {
		request, err := s.readRequest(c)
		if err != nil {
			if request == nil {
				break // 没有请求头,就关闭连接
			}
			request.h.Error = err.Error()                     // 返回解析请求头的错误给客户端,客户端会根据此属性判断
			s.sendResponse(c, request.h, struct{}{}, sending) // 发送错误的信息响应
			continue                                          // 解析请求头失败
		}
		wg.Add(1)
		go s.handleRequest(c, request, sending, wg, opt.HandleTimeout)
	}
	wg.Wait()
	_ = c.Close()
}

type request struct {
	h          *codec.Header
	arg, reply reflect.Value
	mType      *methodType
	svc        *service
}

func (s *Server) readRequest(c codec.Codec) (*request, error) {
	var header codec.Header
	var err error
	if err = c.ReadHeader(&header); err != nil { // 解析请求头
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println("RPC [ReadRequest] Read Header error: ", err)
		}
		return nil, err
	}
	req := &request{h: &header}
	req.svc, req.mType, err = s.findService(header.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.arg = req.mType.newArgv()
	req.reply = req.mType.newReply()
	argvi := req.arg.Interface()
	if req.arg.Type().Kind() != reflect.Pointer { // arg转为指针类型
		argvi = req.arg.Addr().Interface()
	}
	if err := c.ReadBody(argvi); err != nil {
		log.Println("RPC [ReadRequest] Read arg error: ", err)
		return req, err
	}
	return req, nil
}

// 服务端输出请求的响应到客户端
func (s *Server) handleRequest(c codec.Codec, r *request, sending *sync.Mutex, wg *sync.WaitGroup, handleTimeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{}) // 表示执行是否堵塞
	sent := make(chan struct{})
	go func() {
		err := r.svc.call(r.mType, r.arg, r.reply)
		called <- struct{}{} // 执行到这里说明反射执行方法没有堵塞
		if err != nil {
			r.h.Error = err.Error()
			s.sendResponse(c, r.h, errors.New(fmt.Sprintf("RPC server call method error: %s", err)), sending)
			sent <- struct{}{}
			return
		}
		s.sendResponse(c, r.h, r.reply.Interface(), sending)
		sent <- struct{}{}
	}()
	if handleTimeout == 0 {
		<-called
		<-sent
		return
	}
	select {
	case <-time.After(handleTimeout):
		r.h.Error = fmt.Sprintf("RPC [Server] handle timeout: expect within %s", handleTimeout)
		s.sendResponse(c, r.h, struct{}{}, sending)
	case <-called: // 进入这里说明反射执行没有问题
		<-sent
	}

}

// 调用codec的实现类的write的方法输出
func (s *Server) sendResponse(c codec.Codec, h *codec.Header, body any, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := c.Write(h, body); err != nil {
		log.Println("RPC [SendResponse] Write Response error: ", err)
	}
}

// Register 将结构体注册为service到server属性中
func (s *Server) Register(structure any) error {
	service := newService(structure)
	if _, dup := s.serviceMap.LoadOrStore(service.name, service); dup {
		return errors.New("RPC: service already defined: " + service.name)
	}
	return nil
}

func Register(structure any) error {
	return DefaultServer.Register(structure)
}

// serviceMethod: Foo.Sum
func (s *Server) findService(serviceMethod string) (svc *service, mType *methodType, err error) {
	m := strings.LastIndex(serviceMethod, ".")
	if m < 0 {
		err = errors.New("RPC server: service/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:m], serviceMethod[m+1:]
	svc1, ok := s.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("RPC server: can't find service " + serviceName)
		return
	}
	svc = svc1.(*service)
	mType = svc.method[methodName]
	if mType == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}

const (
	Connected        = "200 Connected to RPC"
	DefaultRpcPath   = "/_rpc_"
	DefaultDebugPath = "/debug/rpc"
)

// ServerHttp 基于HTTP的connect连接,然后再传递options
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack() // 获取底层实际的连接
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.0 "+Connected+"\n\n")
	s.serverConn(conn)
}

func (s *Server) HandleHTTP() {
	http.Handle(DefaultRpcPath, s) // 注册路由
}

func HandleHTTP() {
	DefaultServer.HandleHTTP()
}
