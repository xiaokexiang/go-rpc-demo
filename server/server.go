package server

import (
	"encoding/json"
	"fmt"
	"go-rpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c // 魔数

/*
  1. Option 固定使用json来序列化，至于body和header如何序列化由codec.Type决定
  2. | option | Header1 | body1 | Header2 | body2 | ...
*/

type Option struct {
	MagicNumber int        // 表明这是一个rpc请求
	CodecType   codec.Type // client使用何种方式来对body进行编码
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.JsonType,
}

type Server struct{}

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
	server.serverCodec(f(conn))
}

var invalidRequest = struct{}{}

// 一次连接可能有多个请求，所以需要for循环等待，直到错误发生退出
func (server *Server) serverCodec(f codec.Codec) {
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
		go server.handleRequest(f, req, sending, wg) // 协程处理
	}
	wg.Wait()
	_ = f.Close()
}

type request struct {
	h           *codec.Header // 请求的header
	argv, reply reflect.Value // 请求的参数和响应参数
}

// 解析header
func (server *Server) readRequestHeader(c codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := c.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
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
	// 判断参数类型，这里需要使用反射，暂时先考虑为string
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = c.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server -> read argv ")
	}
	return req, nil
}

func (server *Server) sendResponse(c codec.Codec, h *codec.Header, r any, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()

	if err := c.Write(h, r); err != nil { // 加锁依次输出响应
		log.Println("rpc server: write response error: ", err)
	}
}

func (server *Server) handleRequest(f codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	//log.Println(req.h, req.argv.Elem())
	req.reply = reflect.ValueOf(fmt.Sprintf("go-rpc resp %d", req.h.Seq)) // 返回序列号作为响应
	server.sendResponse(f, req.h, req.reply.Interface(), sending)
}
