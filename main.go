package main

import (
	"context"
	"go-rpc/client"
	"go-rpc/server"
	"go-rpc/service"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

func main() {
	addr := make(chan string)
	protocol := "http" // or tcp
	go startClient(protocol, addr)
	startServer(protocol, addr)
}

func startServer(protocol string, addr chan string) {
	var foo service.Foo
	listen, _ := net.Listen("tcp", ":0")
	s := server.DefaultServer
	_ = s.Register(&foo)
	log.Println("rpc server start at: ", listen.Addr().String())
	addr <- listen.Addr().String()
	switch protocol {
	case "http":
		_ = http.Serve(listen, s)
	default:
		s.Accept(listen)
	}
}

func startClient(protocol string, addr chan string) {
	var c *client.Client
	switch protocol {
	case "http":
		c, _ = client.DialHTTP("tcp", <-addr, nil)
	default:
		c, _ = client.Dial("tcp", <-addr, nil)
	}
	defer c.Close()
	time.Sleep(time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(a int) {
			defer wg.Done()
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
