package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	logs "github.com/eru-tech/eru/eru-logs/eru-logs"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdCache is an etcd-backed cache implementation.
type EtcdCache struct {
	CacheStore
	EtcdEndpoints string `json:"etcd_endpoints" eru:"required"`
	Client        *clientv3.Client
}

// NewEtcdCache creates and configures a new etcd cache client.
func NewEtcdCache() (ec *EtcdCache, err error) {
	ec = &EtcdCache{
		CacheStore: CacheStore{CacheStoreType: "ETCD"},
	}
	etcdEndpoints := os.Getenv("ETCD_ENDPOINTS")
	if etcdEndpoints == "" {
		err = logs.Err(context.Background(), fmt.Errorf("ETCD_ENDPOINTS environment variable not set (comma-separated list)"), "")

		return
	}
	ec.EtcdEndpoints = etcdEndpoints
	err = ec.Init(context.Background())
	if err != nil {
		return
	}
	return
}

func (ec *EtcdCache) Init(ctx context.Context) (err error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(ec.EtcdEndpoints, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		err = logs.Err(ctx, err, "failed to connect to etcd")
		return
	}
	// Simple check to see if the connection is alive
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = cli.Status(ctx, cli.Endpoints()[0])
	if err != nil {
		err = logs.Err(ctx, err, "failed to get etcd status")
		return
	}
	ec.Client = cli
	return
}

func (ec *EtcdCache) Get(ctx context.Context, key string) (string, error) {
	resp, err := ec.Client.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return string(resp.Kvs[0].Value), nil
}

func (ec *EtcdCache) Set(ctx context.Context, key string, value interface{}) error {
	return ec.SetWithTTL(ctx, key, value, 0)
}

func (ec *EtcdCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	p, err := json.Marshal(value)
	if err != nil {
		return err
	}
	valStr := string(p)

	if ttl > 0 {
		lease, err := ec.Client.Grant(ctx, int64(ttl.Seconds()))
		if err != nil {
			return err
		}
		_, err = ec.Client.Put(ctx, key, valStr, clientv3.WithLease(lease.ID))
		return err
	}

	_, err = ec.Client.Put(ctx, key, valStr)
	return err
}

func (ec *EtcdCache) GetKeys(ctx context.Context, pattern string) ([]string, error) {
	// etcd uses prefix matching, so we'll treat the pattern as a prefix.
	// This is a simplification; a full glob-style match would be more complex.
	prefix := strings.TrimRight(pattern, "*")
	resp, err := ec.Client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		keys[i] = string(kv.Key)
	}
	return keys, nil
}

func (ec *EtcdCache) Delete(ctx context.Context, key string) error {
	_, err := ec.Client.Delete(ctx, key)
	return err
}
