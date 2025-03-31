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
	"sync"
)

type Option struct {
	MagicNum  int        // mark this is a rpc request
	CodecType codec.Type // choose which codec to encode body
}

const MagicNum = 0x3bef5c

// DefaultOption
// | Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
// | <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|
// | Option | Header1 | Body1 | Header2 | Body2 | ...
var DefaultOption = &Option{
	MagicNum:  MagicNum,
	CodecType: codec.GobType,
}

// Accept accepts connections on the listener and serves requests
func Accept(listen net.Listener) {
	defaultServer.accept(listen)
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

var defaultServer = &Server{}

type Server struct {
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
	s.serverCodec(f(conn))
}

/*
1. 解析请求 不符合请求 发送响应 2. 处理请求
*/
func (s *Server) serverCodec(c codec.Codec) {
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
		go s.handleRequest(c, request, sending, wg)
	}
	wg.Wait()
	_ = c.Close()
}

type request struct {
	h          *codec.Header
	arg, reply reflect.Value
}

func (s *Server) readRequest(c codec.Codec) (*request, error) {
	var header codec.Header
	if err := c.ReadHeader(&header); err != nil { // 解析请求头
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println("RPC [ReadRequest] Read Header error: ", err)
		}
		return nil, err
	}
	req := &request{h: &header}
	req.arg = reflect.New(reflect.TypeOf(""))
	if err := c.ReadBody(req.arg.Interface()); err != nil {
		log.Println("RPC [ReadRequest] Read arg error: ", err)
	}
	return req, nil
}

// 服务端输出请求的响应到客户端
func (s *Server) handleRequest(c codec.Codec, r *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(r.h, r.arg.Elem())
	r.reply = reflect.ValueOf(fmt.Sprintf("RPC resp: %d", r.h.Seq))
	s.sendResponse(c, r.h, r.reply.Interface(), sending)
}

// 调用codec的实现类的write的方法输出
func (s *Server) sendResponse(c codec.Codec, h *codec.Header, body any, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := c.Write(h, body); err != nil {
		log.Println("RPC [SendResponse] Write Response error: ", err)
	}
}
