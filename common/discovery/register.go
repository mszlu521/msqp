package discovery

import (
	"common/config"
	"common/logs"
	"context"
	"encoding/json"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

// Register grpc服务注册到etcd
// 原理：创建一个租约，grpc服务注册到etcd，绑定租约
// 过了租约时间，etcd就会删除grpc服务信息
// 实现心跳，完成续租，如果etcd没有 就新注册
type Register struct {
	etcdCli     *clientv3.Client                        //etcd连接
	leaseId     clientv3.LeaseID                        //租约id
	DialTimeout int                                     //超时时间
	ttl         int                                     //租约时间
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse //心跳
	info        Server                                  //注册的server信息
	closeCh     chan struct{}
}

func NewRegister() *Register {
	return &Register{
		DialTimeout: 3,
	}
}

func (r *Register) Close() {
	r.closeCh <- struct{}{}
}
func (r *Register) Register(conf config.EtcdConf) error {
	//注册信息
	info := Server{
		Name:    conf.Register.Name,
		Addr:    conf.Register.Addr,
		Weight:  conf.Register.Weight,
		Version: conf.Register.Version,
		Ttl:     conf.Register.Ttl,
	}
	//建立etcd的连接
	var err error
	r.etcdCli, err = clientv3.New(clientv3.Config{
		Endpoints:   conf.Addrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	})
	if err != nil {
		return err
	}
	r.info = info
	if err = r.register(); err != nil {
		return err
	}
	r.closeCh = make(chan struct{})
	//放入协程中 根据心跳的结果 做相应的操作
	go r.watcher()
	return nil
}

func (r *Register) register() error {
	//1. 创建租约
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(r.DialTimeout))
	defer cancel()
	var err error
	if err = r.createLease(ctx, r.info.Ttl); err != nil {
		return err
	}
	//2. 心跳检测
	if r.keepAliveCh, err = r.keepAlive(); err != nil {
		return err
	}
	//3. 绑定租约
	data, _ := json.Marshal(r.info)
	//key value
	return r.bindLease(ctx, r.info.BuildRegisterKey(), string(data))
}

// createLease ttl秒
func (r *Register) bindLease(ctx context.Context, key, value string) error {
	//put动作
	_, err := r.etcdCli.Put(ctx, key, value, clientv3.WithLease(r.leaseId))
	if err != nil {
		logs.Error("bindLease failed,err:%v", err)
		return err
	}
	logs.Info("register service success,key=%s", key)
	return nil
}

// createLease ttl秒
func (r *Register) createLease(ctx context.Context, ttl int64) error {
	grant, err := r.etcdCli.Grant(ctx, ttl)
	if err != nil {
		logs.Error("createLease failed,err:%v", err)
		return err
	}
	r.leaseId = grant.ID
	return nil
}

// keepAlive 心跳检测
func (r *Register) keepAlive() (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	//心跳 要求是一个长连接 如果做了超时 长连接就断掉了 不要设置超时
	//就是一直不停的发消息 保持租约 续租
	keepAliveResponses, err := r.etcdCli.KeepAlive(context.Background(), r.leaseId)
	if err != nil {
		logs.Error("keepAlive failed,err:%v", err)
		return keepAliveResponses, err
	}
	return keepAliveResponses, nil
}

// watcher 续约 新注册 close 注销
func (r *Register) watcher() {
	//租约到期了 是不是需要去检查是否自动注册
	ticker := time.NewTicker(time.Duration(r.info.Ttl) * time.Second)
	for {
		select {
		case <-r.closeCh:
			if err := r.unregister(); err != nil {
				logs.Error("close and unregister failed,err:%v", err)
			}
			//租约撤销
			if _, err := r.etcdCli.Revoke(context.Background(), r.leaseId); err != nil {
				logs.Error("close and Revoke lease failed,err:%v", err)
			}
			if r.etcdCli != nil {
				r.etcdCli.Close()
			}
			logs.Info("unregister etcd...")
		case res := <-r.keepAliveCh:
			//如果etcd重启了 相当于连接断开 需要进行重新连接 res==nil
			if res == nil {
				if err := r.register(); err != nil {
					logs.Error("keepAliveCh register failed,err:%v", err)
				}
				logs.Info("续约重新注册成功,%v", res)
			}
		case <-ticker.C:
			if r.keepAliveCh == nil {
				if err := r.register(); err != nil {
					logs.Error("ticker register failed,err:%v", err)
				}
			}
		}
	}
}

func (r *Register) unregister() error {
	_, err := r.etcdCli.Delete(context.Background(), r.info.BuildRegisterKey())
	return err
}
