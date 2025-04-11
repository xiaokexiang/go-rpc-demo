package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go-rpc/client"
	"go-rpc/codec"
	"go-rpc/server"
	_struct "go-rpc/struct"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

func startServer(addr chan string, h bool) {
	var foo _struct.Foo
	err := server.Register(&foo)
	if err != nil {
		log.Fatal("register error:", err)
	}
	listen, err := net.Listen("tcp", ":0") // 表示选择随即端口
	if err != nil {
		log.Fatal("Start Server error: ", err)
	}
	log.Println("Start Server Success on: ", listen.Addr())
	addr <- listen.Addr().String() // send addr to client
	if h {
		server.HandleHTTP()
		_ = http.Serve(listen, nil)
	} else {
		server.Accept(listen)
	}
}

func sendMessage(conn io.ReadWriteCloser) {
	_ = json.NewEncoder(conn).Encode(server.DefaultOption)
	c := codec.NewGobCodec(conn)
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.sum",
			Seq:           uint64(i),
		}
		_ = c.Write(h, fmt.Sprintf("Client send message to server: %d", h.Seq)) // client 写入header和body, server先解析header然后解析body
		_ = c.ReadHeader(h)                                                     // 先读取server返回的响应头
		var reply string
		_ = c.ReadBody(&reply) // 再读取server返回的body
		log.Println("Reply from server: ", reply)
	}
}

func call(addr chan string, http bool) {
	var c *client.Client
	if http {
		c, _ = client.DialHTTP("tcp", <-addr)
	} else {
		c, _ = client.Dial("tcp", <-addr)
	}
	defer func() { _ = c.Close() }()
	time.Sleep(time.Second) // wait client connected to server
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &_struct.Args{Num1: i, Num2: i * i}
			var reply int
			ctx, _ := context.WithTimeout(context.Background(), server.DefaultOption.ConnectTimeout) // 基于context实现
			if err := c.SendSync(ctx, "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}

func main() {
	addr := make(chan string)
	go call(addr, false)
	startServer(addr, false) // startServer在后,阻塞主goroutine
}
