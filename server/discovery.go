package server

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int

// 定义随机和轮询2种负载均衡策略
const (
	Random SelectMode = iota + 1
	RoundRobin
)

type Discovery interface {
	Refresh() error
	Update(server []string) error
	Get(mode SelectMode) (string, error)
	GetAll() ([]string, error)
}

type MultiServersDiscovery struct {
	r       *rand.Rand
	mu      sync.Mutex
	servers []string
	index   int // 用做轮询
}

func NewMultiServerDiscovery(server []string) *MultiServersDiscovery {
	d := &MultiServersDiscovery{
		servers: server,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1) // 随机数
	return d
}

var _ Discovery = (*MultiServersDiscovery)(nil) // 用于编译器检查

func (d *MultiServersDiscovery) Refresh() error {
	return nil
}

func (d *MultiServersDiscovery) Update(server []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = server
	return nil
}

func (d *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.servers)
	if mode == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}
	switch mode {
	case Random:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobin:
		s := d.servers[d.index%n] // 取模
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("rpc discovery: not supported select mode")
	}
}

func (d *MultiServersDiscovery) GetAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	servers := make([]string, len(d.servers), len(d.servers))
	copy(servers, d.servers) // 返回副本,避免修改底层数组
	return servers, nil
}
