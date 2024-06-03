package discovery

import (
	"common/config"
	"common/logs"
	"context"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
	"time"
)

type Resolver struct {
	conf        config.EtcdConf
	etcdCli     *clientv3.Client //etcd连接
	DialTimeout int              //超时时间
	closeCh     chan struct{}
	key         string
	cc          resolver.ClientConn
	srvAddrList []resolver.Address
	watchCh     clientv3.WatchChan
}

// Build 当grpc.Dial的时候 就会同步调用此方法
func (r *Resolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	//获取到调用的key（user/v1）连接etcd 获取其value
	r.cc = cc
	//1.连接etcd
	//建立etcd的连接
	var err error
	r.etcdCli, err = clientv3.New(clientv3.Config{
		Endpoints:   r.conf.Addrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	})
	if err != nil {
		logs.Fatal("grpc client connect etcd err:%v", err)
	}
	r.closeCh = make(chan struct{})
	//2.根据key获取value
	r.key = target.URL.Path
	if err = r.sync(); err != nil {
		return nil, err
	}
	//2. 比如节点有变动了 想要实时的更新信息
	go r.watch()
	return nil, nil
}

func (r *Resolver) Scheme() string {
	return "etcd"
}

func (r *Resolver) sync() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.conf.RWTimeout)*time.Second)
	defer cancel()
	// user/v1/xxx:1111
	// user/v1/xxx:2222
	res, err := r.etcdCli.Get(ctx, r.key, clientv3.WithPrefix())
	if err != nil {
		logs.Error("grpc client get etcd failed, name=%s,err:%v", r.key, err)
		return err
	}
	logs.Info("%v", res.Kvs)
	r.srvAddrList = []resolver.Address{}
	for _, v := range res.Kvs {
		server, err := ParseValue(v.Value)
		if err != nil {
			logs.Error("grpc client parse etcd value failed, name=%s,err:%v", r.key, err)
			continue
		}
		r.srvAddrList = append(r.srvAddrList, resolver.Address{
			Addr:       server.Addr,
			Attributes: attributes.New("weight", server.Weight),
		})
	}
	if len(r.srvAddrList) == 0 {
		logs.Error("no services found")
		return nil
	}
	err = r.cc.UpdateState(resolver.State{
		Addresses: r.srvAddrList,
	})
	if err != nil {
		logs.Error("grpc client UpdateState failed, name=%s, err: %v", r.key, err)
		return err
	}
	return nil
}

func (r *Resolver) watch() {
	//1. 定时 1分钟同步一次数据
	//2. 监听节点的事件 从而触发不同的操作
	//3. 监听Close事件 关闭 etcd
	ticker := time.NewTicker(time.Minute)
	r.watchCh = r.etcdCli.Watch(context.Background(), r.key, clientv3.WithPrefix())
	for {
		select {
		case <-r.closeCh:
			//close
			r.Close()
		case res, ok := <-r.watchCh:
			if ok {
				//
				r.update(res.Events)
			}

		case <-ticker.C:
			if err := r.sync(); err != nil {
				logs.Error("watch sync failed,err:%v", err)
			}
		}
	}
}

func (r *Resolver) update(events []*clientv3.Event) {
	for _, ev := range events {
		switch ev.Type {
		case clientv3.EventTypePut:
			//put key value
			server, err := ParseValue(ev.Kv.Value)
			if err != nil {
				logs.Error("grpc client update(EventTypePut) parse etcd value failed, name=%s,err:%v", r.key, err)
			}
			addr := resolver.Address{
				Addr:       server.Addr,
				Attributes: attributes.New("weight", server.Weight),
			}
			if !Exist(r.srvAddrList, addr) {
				r.srvAddrList = append(r.srvAddrList, addr)
				err = r.cc.UpdateState(resolver.State{
					Addresses: r.srvAddrList,
				})
				if err != nil {
					logs.Error("grpc client update(EventTypePut) UpdateState failed, name=%s,err:%v", r.key, err)
				}
			}
		case clientv3.EventTypeDelete:
			//接收到delete操作 删除r.srvAddrList其中匹配的
			// user/v1/127.0.0.1:12000
			server, err := ParseKey(string(ev.Kv.Key))
			if err != nil {
				logs.Error("grpc client update(EventTypeDelete) parse etcd value failed, name=%s,err:%v", r.key, err)
			}
			addr := resolver.Address{Addr: server.Addr}
			//r.srvAddrList remove操作
			if list, ok := Remove(r.srvAddrList, addr); ok {
				r.srvAddrList = list
				err = r.cc.UpdateState(resolver.State{
					Addresses: r.srvAddrList,
				})
				if err != nil {
					logs.Error("grpc client update(EventTypeDelete) UpdateState failed, name=%s,err:%v", r.key, err)
				}
			}
		}
	}
}

func (r *Resolver) Close() {
	if r.etcdCli != nil {
		err := r.etcdCli.Close()
		if err != nil {
			logs.Error("Resolver close etcd err:%v", err)
		}
		logs.Info("close etcd...")
	}
}

func Exist(list []resolver.Address, addr resolver.Address) bool {
	for i := range list {
		if list[i].Addr == addr.Addr {
			return true
		}
	}
	return false
}

func Remove(list []resolver.Address, addr resolver.Address) ([]resolver.Address, bool) {
	for i := range list {
		if list[i].Addr == addr.Addr {
			list[i] = list[len(list)-1]
			return list[:len(list)-1], true
		}
	}
	return nil, false
}
func NewResolver(conf config.EtcdConf) *Resolver {
	return &Resolver{
		conf:        conf,
		DialTimeout: conf.DialTimeout,
	}
}
