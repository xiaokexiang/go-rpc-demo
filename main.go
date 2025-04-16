package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go-rpc/client"
	"go-rpc/codec"
	"go-rpc/registry"
	"go-rpc/server"
	_struct "go-rpc/struct"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Foo int

type Args struct{ Num1, Num2 int }

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

func foo(xc *client.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func startRegistry(wg *sync.WaitGroup) {
	l, _ := net.Listen("tcp", ":9999")
	registry.HandleHTTP()
	wg.Done()
	_ = http.Serve(l, nil)
}

func startServer(registryAddr string, wg *sync.WaitGroup) {
	var foo Foo
	l, _ := net.Listen("tcp", ":0")
	newServer := server.NewServer()
	_ = newServer.Register(&foo)
	registry.Heartbeat(registryAddr, "tcp@"+l.Addr().String(), 0)
	wg.Done()
	newServer.Accept(l)
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

func call2(addr1, addr2 string) {
	d := server.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
	xc := client.NewXClient(d, server.Random, nil)
	defer func() { _ = xc.Close() }()
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(addr1, addr2 string) {
	d := server.NewMultiServerDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
	xc := client.NewXClient(d, server.Random, nil)
	defer func() { _ = xc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
			// expect 2 - 5 timeout
			//ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			//foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func call3(registry string) {
	d := server.NewRegistryDiscovery(registry, 0)
	xc := client.NewXClient(d, server.Random, nil)
	defer func() { _ = xc.Close() }()
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast2(registry string) {
	d := server.NewRegistryDiscovery(registry, 0)
	xc := client.NewXClient(d, server.Random, nil)
	defer func() { _ = xc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})
			// expect 2 - 5 timeout
			//ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			//foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func main() {
	log.SetFlags(0)
	registryAddr := "http://localhost:9999/_geerpc_/registry"
	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	time.Sleep(time.Second)
	wg.Add(2)
	go startServer(registryAddr, &wg)
	go startServer(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call3(registryAddr)
	broadcast2(registryAddr)
}
