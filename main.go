package main

import (
	"fmt"
	"go-rpc/server"
	"log"
	"main/client"
	"net"
	"sync"
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
	c, _ := client.Dial("tcp", <-addr)
	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(a int) { // 迭代变量捕获
			args := fmt.Sprintf("rpc req %d", a)
			var reply string
			if err := c.Sync("Foo.sum", args, &reply); err != nil {
				log.Fatalln("call Foo.sum error: ", err)
			}
			log.Println("reply: ", reply)
		}(i)
	}
	wg.Wait()
}
