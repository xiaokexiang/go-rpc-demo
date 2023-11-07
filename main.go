package main

import (
	"context"
	"go-rpc/client"
	"go-rpc/server"
	"go-rpc/service"
	"log"
	"net"
	"sync"
	"time"
)

func startServer(addr chan string) {
	var foo service.Foo
	s := server.DefaultServer
	if err := s.Register(&foo); err != nil {
		log.Fatalln("rpc server register error: ", err)
	}
	listen, _ := net.Listen("tcp", ":0")
	addr <- listen.Addr().String() // 传输地址给客户端
	s.Accept(listen)               // 服务端监听端口，接受客户端请求
}

func main() {
	addr := make(chan string)
	go startServer(addr)
	c, err := client.Dial("tcp", <-addr, nil)
	if err != nil {
		log.Fatalf("rpc client: connect rpc server error: %s\n", err.Error())
	}
	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(a int) { // 迭代变量捕获
			args := &service.Args{Num1: a, Num2: a * a}
			var reply int
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := c.Sync(ctx, "Foo.Sum", args, &reply); err != nil {
				log.Fatalln("call Foo.sum error: ", err)
			}
			log.Printf("rpc client, result: %d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}
