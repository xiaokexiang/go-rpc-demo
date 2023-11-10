package client

import (
	"context"
	"go-rpc/server"
	"go-rpc/service"
	"log"
	"net"
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
