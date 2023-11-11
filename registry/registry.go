package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DefaultTimeout = 5 * time.Minute
	DefaultPath    = "/rpc/registry"
	DefaultHeader  = "X-Rpc-Servers"
)

var DefaultRegister = NewRegistry(DefaultTimeout)

type ServerItem struct {
	Addr  string    // 注册地址
	start time.Time // 注册时间
}

type Registry struct {
	timeout time.Duration // 注册中心超时时间
	mu      sync.Mutex
	servers map[string]*ServerItem // 注册中心的注册服务列表
}

func NewRegistry(timeout time.Duration) *Registry {
	return &Registry{
		servers: make(map[string]*ServerItem),
		timeout: timeout,
	}
}

// 注册服务到注册中心
func (r *Registry) registerServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	server, ok := r.servers[addr]
	if ok {
		server.start = time.Now() // 存在就更新注册时间
	} else {
		r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()}
	}
}

// 返回可用的服务
func (r *Registry) aliveServer() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || time.Now().Before(s.start.Add(r.timeout)) {
			alive = append(alive, addr)
		} else {
			// 从注册中心移除
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

// Heartbeat 发送心跳并更新注册时间
func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		duration = DefaultTimeout - time.Duration(1)*time.Minute // 保证足够的时间发送心跳
	}
	var err error
	err = sendHeartbeat(registry, addr) // 预先发送一次心跳
	go func() {
		t := time.NewTicker(duration)
		for err == nil {
			<-t.C
			err = sendHeartbeat(registry, addr)
		}
	}()
}

func sendHeartbeat(registry, addr string) error {
	c := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set(DefaultHeader, addr)
	if _, err := c.Do(req); err != nil {
		log.Println("rpc server: heartbeat error:", err)
		return err
	}
	return nil
}

// ServeHttp GET 获取所有可用的服务列表， POST 注册服务到注册中心
func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.Header().Set(DefaultHeader, strings.Join(r.aliveServer(), ","))
	case "POST":
		addr := req.Header.Get(DefaultHeader)
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.registerServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
