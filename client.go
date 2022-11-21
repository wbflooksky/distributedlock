package distributedlock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis/v9"
)

var (
	ErrRedisNil = errors.New("redis: nil")
)

// Scripter refers to the redis.scripter interface.
type RedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (int64, error)
	// BRPop
	// - ret:
	//   - error:
	//     - nil
	//     - redis.Nil: 列表无数据
	//     - 其他错误
	BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error)
}

type redisClient struct {
	*redis.ClusterClient
	scriptMap map[string]*redis.Script
}

// NewRedisClient 提供了一个基于redis/v9版本的RedisClient接口实现
// 使用方可以参考该示例
// 实现自己的接口，创建分布式锁实例
func NewRedisClient(client *redis.ClusterClient) RedisClient {
	return &redisClient{
		ClusterClient: client,
		scriptMap: map[string]*redis.Script{
			lockLua:    redis.NewScript(lockLua),
			unlockLua:  redis.NewScript(unlockLua),
			renewalLua: redis.NewScript(renewalLua),
		},
	}
}

func (impl *redisClient) BRPop(ctx context.Context, timeout time.Duration, keys ...string) ([]string, error) {
	values, err := impl.ClusterClient.BRPop(context.Background(), timeout, keys...).Result()
	switch err {
	case redis.Nil:
		return nil, ErrRedisNil
	case nil:
		return values, nil
	default:
		return nil, err
	}
}

func (impl *redisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (int64, error) {
	scripter, exist := impl.scriptMap[script]
	if !exist {
		return 0, fmt.Errorf("unregistered script")
	}
	return scripter.Run(ctx, impl.ClusterClient, keys, args...).Int64()
}
