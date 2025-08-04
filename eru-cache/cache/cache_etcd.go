package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdCache is an etcd-backed cache implementation.
type EtcdCache struct {
	CacheStore
	Client *clientv3.Client
}

// NewEtcdCache creates and configures a new etcd cache client.
func NewEtcdCache() (*EtcdCache, error) {
	etcdEndpoints := os.Getenv("ETCD_ENDPOINTS")
	if etcdEndpoints == "" {
		return nil, errors.New("ETCD_ENDPOINTS environment variable not set (comma-separated list)")
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(etcdEndpoints, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}
	// Simple check to see if the connection is alive
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = cli.Status(ctx, cli.Endpoints()[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd status: %w", err)
	}
	return &EtcdCache{
		CacheStore: CacheStore{CacheStoreType: "ETCD"},
		Client:     cli,
	}, nil
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