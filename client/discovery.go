package client

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

const (
	RandomSelect SelectMode = iota
	RoundRobinSelect
)

type SelectMode int
type Discovery interface {
	Refresh() error                      //从注册中心更新服务列表
	Update(servers []string) error       // 手动更新服务列表
	Get(mode SelectMode) (string, error) // 根据负载策略选择一个服务实例
	GetAll() ([]string, error)           // 获取所有的服务实例
}

type MultiServerDiscovery struct {
	r       *rand.Rand
	mu      sync.RWMutex
	servers []string
	index   int // 记录robin轮循的位置
}

func NewMultiServerDiscovery(servers []string) *MultiServerDiscovery {
	d := &MultiServerDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1) // 初始化随机指定一个值
	return d
}

func (d *MultiServerDiscovery) Refresh() error {
	return nil // 暂时没有注册中心，空实现
}

func (d *MultiServerDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	return nil
}

// Get 通过SelectMode选择一个服务实例
func (d *MultiServerDiscovery) Get(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	n := len(d.servers)
	if n == 0 {
		return "", errors.New("rpc discovery: no available servers")
	}

	switch mode {
	case RandomSelect:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobinSelect:
		s := d.servers[d.index%n]
		d.index = (d.index + 1) % n // 更新index
		return s, nil
	default:
		return "", errors.New("rpc discovery: not support select mode")
	}
}

func (d *MultiServerDiscovery) GetAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	servers := make([]string, len(d.servers))
	copy(servers, d.servers) // 返回副本，不改变切片引用的底层数组
	return servers, nil
}
