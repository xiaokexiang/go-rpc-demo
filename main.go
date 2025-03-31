package main

import (
	"encoding/json"
	"fmt"
	"go-rpc/client"
	"go-rpc/codec"
	"go-rpc/server"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

func startServer(addr chan string) {
	listen, err := net.Listen("tcp", ":0") // 表示选择随即端口
	if err != nil {
		log.Fatal("Start Server error: ", err)
	}
	log.Println("Start Server Success on: ", listen.Addr())
	addr <- listen.Addr().String() // send addr to client
	server.Accept(listen)
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

func main() {
	addr := make(chan string)
	go startServer(addr)
	c, _ := client.Dial("tcp", <-addr)
	defer func() { _ = c.Close() }()
	time.Sleep(time.Second) // wait client connected to server
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("Client Req %d", i)
			var reply string
			if err := c.SendSync("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}
