package zk

import (
	"time"

	"github.com/go-zookeeper/zk"
)

type zkConn interface {
	AddAuth(scheme string, auth []byte) error
	Exists(path string) (bool, *zk.Stat, error)
	Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error)
	Delete(path string, version int32) error
	Get(path string) ([]byte, *zk.Stat, error)
	Children(path string) ([]string, *zk.Stat, error)
	ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error)
	CreateProtectedEphemeralSequential(path string, data []byte, acl []zk.ACL) (string, error)
	Close()
}

func connectZk(servers []string, sessionTimeout time.Duration, opts ...zkConnOpt) (zkConn, error) {
	opt := newZkOptions(opts...)
	var conn *zk.Conn
	var err error
	if opt.ecb != nil {
		conn, _, err = zk.Connect(servers, sessionTimeout, zk.WithEventCallback(opt.ecb))
	} else {
		conn, _, err = zk.Connect(servers, sessionTimeout)
	}
	if err != nil {
		return nil, err
	}
	return &zkConnImpl{conn: conn}, nil
}

type zkConnImpl struct {
	conn *zk.Conn
}

func (impl *zkConnImpl) AddAuth(scheme string, auth []byte) error {
	return impl.conn.AddAuth(scheme, auth)
}

func (impl *zkConnImpl) Exists(path string) (bool, *zk.Stat, error) {
	return impl.conn.Exists(path)
}

func (impl *zkConnImpl) Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error) {
	return impl.conn.Create(path, data, flags, acl)
}

func (impl *zkConnImpl) Delete(path string, version int32) error {
	return impl.conn.Delete(path, version)
}

func (impl *zkConnImpl) Get(path string) ([]byte, *zk.Stat, error) {
	return impl.conn.Get(path)
}

func (impl *zkConnImpl) Children(path string) ([]string, *zk.Stat, error) {
	return impl.conn.Children(path)
}

func (impl *zkConnImpl) ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
	return impl.conn.ChildrenW(path)
}

func (impl *zkConnImpl) CreateProtectedEphemeralSequential(path string, data []byte, acl []zk.ACL) (string, error) {
	return impl.conn.CreateProtectedEphemeralSequential(path, data, acl)
}

func (impl *zkConnImpl) Close() {
	impl.conn.Close()
}

type zkoption struct {
	ecb zk.EventCallback
}

func newZkOptions(opts ...zkConnOpt) *zkoption {
	opt := &zkoption{}
	for _, fn := range opts {
		fn(opt)
	}
	return opt
}

type zkConnOpt func(*zkoption)

func WithEventCallback(cb zk.EventCallback) zkConnOpt {
	return func(o *zkoption) {
		o.ecb = cb
	}
}
