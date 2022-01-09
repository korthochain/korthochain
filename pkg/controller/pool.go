package controller

import (
	"sync"
	"time"

	"google.golang.org/grpc"
)

type clientInfo struct {
	c   *grpc.ClientConn
	num int
}

type pool struct {
	sync.Mutex
	Cap int
	Len int
	m   map[string]*clientInfo
}

func newPool() *pool {
	return &pool{
		Cap: 5,
		Len: 0,
		m:   make(map[string]*clientInfo, 10),
	}
}

func (p *pool) GetClient(url string) (*grpc.ClientConn, error) {
	p.Lock()
	defer p.Unlock()

	v, ok := p.m[url]
	if ok {
		p.m[url].num++
		return v.c, nil
	}

	cc, err := grpc.Dial(url, grpc.WithInsecure(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}

	if p.Len < p.Cap {
		p.m[url] = &clientInfo{cc, 1}
	} else {
		p.updatePool()
		p.m[url] = &clientInfo{cc, 1}
	}
	p.Len++

	//TODO:
	return cc, nil
}

func (p *pool) PutClient(url string, cc *grpc.ClientConn) {
	p.Lock()
	defer p.Unlock()

	if _, ok := p.m[url]; ok {
		p.m[url].num--
		return
	} else if p.Len < p.Cap {
		p.m[url] = &clientInfo{cc, 0}
		return
	}

	cc.Close()
}

func (p *pool) updatePool() {
	minK := ""
	minNum := int(^uint32(0) >> 1)

	delKeyList := make([]string, p.Len)
	for k, v := range p.m {
		if v.num == 0 {
			delKeyList = append(delKeyList, k)
		} else {
			if v.num < minNum {
				minNum = v.num
				minK = k
			}
		}
	}

	for _, k := range delKeyList {
		delete(p.m, k)
		p.Len--
	}

	if p.Len >= p.Cap {
		delete(p.m, minK)
		p.Len--
	}
}
