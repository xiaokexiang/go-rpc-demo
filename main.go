package main

import (
	"encoding/json"
	"fmt"
	"go-rpc/codec"
	"go-rpc/server"
	"log"
	"net"
	"time"
)

func startServer(addr chan string) {
	listen, _ := net.Listen("tcp", ":0")
	addr <- listen.Addr().String()    // 传输地址给客户端
	server.NewServer().Accept(listen) // 服务端监听端口，接受客户端请求
}

func main() {
	addr := make(chan string)
	go startServer(addr)
	conn, _ := net.Dial("tcp", <-addr)
	defer func() {
		_ = conn.Close()
	}()
	time.Sleep(time.Second)

	_ = json.NewEncoder(conn).Encode(server.DefaultOption) // 输出option到服务端
	c := codec.NewJsonCodec(conn)

	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		send := fmt.Sprintf("go-rpc req %d", h.Seq)
		log.Println("client send msg: ", send)
		_ = c.Write(h, send)
		_ = c.ReadHeader(h) // 需要先消费返回的header，因为write是先输出header后输出body的
		var reply string
		_ = c.ReadBody(&reply)
		log.Println("server reply:", reply)
	}
}
