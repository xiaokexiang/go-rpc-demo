package client

import (
	"context"
	"go-rpc/registry"
	"go-rpc/server"
	"go-rpc/service"
	"log"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func Test(t *testing.T) {
	ch1 := make(chan string)
	ch2 := make(chan string)

	go startServer(ch1)
	go startServer(ch2)

	addr1 := <-ch1
	addr2 := <-ch2

	time.Sleep(time.Second)
	call(addr1, addr2)
	broadcast(addr1, addr2)
}

func foo(c *LoadBalanceClient, ctx context.Context, typ, serviceMethod string, args *service.Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = c.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = c.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v\n", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d\n", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

func call(address ...string) {
	d := NewMultiServerDiscovery(address)
	c := NewLoadBalanceClient(d, RandomSelect, nil)
	defer func() { _ = c.Close() }()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(c, context.Background(), "call", "Foo.Sum", &service.Args{Num1: i, Num2: i * i})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			defer cancel()
			foo(c, ctx, "call", "Foo.Sleep", &service.Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(address ...string) {
	d := NewMultiServerDiscovery(address)
	c := NewLoadBalanceClient(d, RandomSelect, nil)
	defer func() { _ = c.Close() }()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(c, context.Background(), "call", "Foo.Sum", &service.Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func startServer(addr chan string) {
	var foo service.Foo
	l, _ := net.Listen("tcp", ":0")
	s := server.DefaultServer
	_ = s.Register(&foo)
	log.Printf("server listen on port: %s\n", l.Addr().String())
	addr <- l.Addr().String()
	s.Accept(l)
}

func Test2(t *testing.T) {
	registryAddr := "http://localhost:9999/_geerpc_/registry"
	var wg sync.WaitGroup
	wg.Add(1)
	go startRegistry(&wg)
	wg.Wait()

	time.Sleep(time.Second)
	wg.Add(2)
	go startServer3(registryAddr, &wg)
	go startServer3(registryAddr, &wg)
	wg.Wait()

	time.Sleep(time.Second)
	call2(registryAddr)
	broadcast2(registryAddr)
}

func startRegistry(wg *sync.WaitGroup) {
	listen, _ := net.Listen("tcp", ":9999")
	wg.Done()
	_ = http.Serve(listen, registry.DefaultRegister)
}

func startServer3(addr string, wg *sync.WaitGroup) {
	var foo service.Foo
	listen, _ := net.Listen("tcp", ":0")
	s := server.NewServer()
	_ = s.Register(&foo)
	registry.Heartbeat(addr, listen.Addr().String(), 0)
	wg.Done()
	s.Accept(listen)
}

func call2(registry string) {
	d := NewRegistryDiscovery(registry, 0)
	xc := NewLoadBalanceClient(d, RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "Foo.Sum", &service.Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast2(registry string) {
	d := NewRegistryDiscovery(registry, 0)
	xc := NewLoadBalanceClient(d, RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "Foo.Sum", &service.Args{Num1: i, Num2: i * i})
			// expect 2 - 5 timeout
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			foo(xc, ctx, "broadcast", "Foo.Sleep", &service.Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}
