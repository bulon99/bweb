package rpc

import (
	"context"
	"github.com/bulon99/msgo/register"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"net"
	"time"
)

//服务端封装
type MsGrpcServer struct {
	listen    net.Listener
	server    *grpc.Server
	registers []func(g *grpc.Server)
}

func NewGrpcServer(address string, ops ...grpc.ServerOption) (*MsGrpcServer, error) {
	listen, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	ms := &MsGrpcServer{
		listen: listen,
	}
	server := grpc.NewServer(ops...)
	ms.server = server
	return ms, nil
}

func (s *MsGrpcServer) Run() error {
	for _, regis := range s.registers {
		regis(s.server)
	}
	return s.server.Serve(s.listen)
}

func (s *MsGrpcServer) Stop() {
	s.server.Stop()
}

func (s *MsGrpcServer) Register(reg func(grpServer *grpc.Server), serviceName, host string, port uint64, Type string) {
	s.registers = append(s.registers, reg)
	//注册到nacos
	if Type == "nacos" {
		client, err := register.CreateNacosClient()
		if err != nil {
			panic(err)
		}
		err = register.RegService(client, serviceName, host, port)
		if err != nil {
			panic(err)
		}
	} else if Type == "etcd" {
		//注册到etcd
		etcdClient, err := register.CreateEtcdCli(register.DefaultOption)
		if err != nil {
			panic(err)
		}
		err = register.RegEtcdService(etcdClient, serviceName, host, port)
		if err != nil {
			panic(err)
		}
		etcdClient.Close()
	}
}

//客户端封装
type MsGrpcClient struct {
	Conn *grpc.ClientConn
}

func NewGrpcClient(config *MsGrpcClientConfig) (*MsGrpcClient, error) {
	var ctx = context.Background()
	var dialOptions = config.DialOptions

	if config.Block {
		//阻塞
		if config.DialTimeout > time.Duration(0) {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, config.DialTimeout) //超出时间时会自动cancel
			defer cancel()
		}
		dialOptions = append(dialOptions, grpc.WithBlock())
	}
	if config.KeepAlive != nil {
		dialOptions = append(dialOptions, grpc.WithKeepaliveParams(*config.KeepAlive))
	}
	conn, err := grpc.DialContext(ctx, config.Address, dialOptions...)
	if err != nil {
		return nil, err
	}
	return &MsGrpcClient{
		Conn: conn,
	}, nil
}

type MsGrpcClientConfig struct {
	Address     string
	Block       bool
	DialTimeout time.Duration
	ReadTimeout time.Duration
	Direct      bool
	KeepAlive   *keepalive.ClientParameters
	DialOptions []grpc.DialOption
}

func DefaultGrpcClientConfig() *MsGrpcClientConfig {
	return &MsGrpcClientConfig{
		DialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
		DialTimeout: time.Second * 3,
		ReadTimeout: time.Second * 2,
		Block:       true,
	}
}
