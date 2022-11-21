package distributedlock

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	_ "embed"

	"github.com/google/uuid"
)

var (
	//go:embed lock.lua
	lockLua string
	//go:embed unlock.lua
	unlockLua string
	//go:embed renewal.lua
	renewalLua string
)

const (
	blockQueueDeferTimeSecond int64         = 5
	lockExpireSplitSection    int64         = 4
	minLockExpire             time.Duration = time.Second * time.Duration(lockExpireSplitSection)
	blockQueueKeyword         string        = "blockKey"
)

type redisDistributedLocker struct {
	hashTag string
	client  RedisClient
	expire  int64
}

var _ DistributedLocker = (*redisDistributedLocker)(nil)

type redisDistributedUnLocker struct {
	client                RedisClient
	lockKey, waitQueueKey string
	lockUniqueID          string
	expire                int64
	keySetTime            time.Time
	status                int32
	err                   atomic.Value
}

var _ UnLocker = (*redisDistributedUnLocker)(nil)

func init() {
	lockLua = strings.TrimSpace(lockLua)
	unlockLua = strings.TrimSpace(unlockLua)
	renewalLua = strings.TrimSpace(renewalLua)
}

// NewDistributedLockWithRedis ...
// namespace: hash tag, compatible transaction redis cluster
// expire: lock expire time, the time is accurate to the second. The value must be no less than four seconds
func NewDistributedLockWithRedis(namespace string, expire time.Duration, client RedisClient) (DistributedLocker, error) {
	if expire < minLockExpire {
		return nil, ErrLockExpireTooShort
	}
	return &redisDistributedLocker{
		client:  client,
		hashTag: namespace,
		expire:  int64(expire / time.Second),
	}, nil
}

func (impl *redisDistributedLocker) Lock(key string) (UnLocker, error) {
	if err := impl.verifyKey(key); err != nil {
		return nil, err
	}
	uniqueID := uuid.New().String()
	lockKey, waitQueueKey := impl.key(key), impl.blockKey(key)
	result, err := impl.client.Eval(context.Background(), lockLua, []string{lockKey, waitQueueKey}, uniqueID, impl.expire)
	if err != nil {
		return nil, err
	}
	if result == 0 {
		return nil, ErrLocked
	}
	unLocker := &redisDistributedUnLocker{
		client:       impl.client,
		lockKey:      lockKey,
		waitQueueKey: waitQueueKey,
		lockUniqueID: uniqueID,
		expire:       impl.expire,
		keySetTime:   time.Now(),
	}
	unLocker.startRenewal()
	return unLocker, nil
}

func (impl *redisDistributedLocker) TryLock(key string, timeout time.Duration) (UnLocker, error) {
	acquireLockTime := time.Now()
	waitQueueKey := impl.blockKey(key)
	for {
		unlocker, err := impl.Lock(key)
		switch err {
		case nil:
			return unlocker, nil
		case ErrLocked:
			waitTime := timeout - time.Since(acquireLockTime)
			if waitTime <= 0 {
				return nil, ErrLockWaitTimeout
			}
			_, err := impl.client.BRPop(context.Background(), waitTime, waitQueueKey)
			switch err {
			case ErrRedisNil:
			case nil:
			default:
				return nil, err
			}
		default:
			return nil, err
		}
	}
}

func (impl *redisDistributedUnLocker) UnLock() error {
	if impl.isUnlocked() {
		return ErrUnlocked
	}
	result, err := impl.client.Eval(context.Background(),
		unlockLua,
		[]string{impl.lockKey, impl.waitQueueKey},
		impl.lockUniqueID,
		blockQueueDeferTimeSecond,
	)
	if err != nil {
		return err
	}
	impl.openUnlockFlag()
	if result == 0 {
		return ErrUnlocked
	}
	return nil
}

func (impl *redisDistributedUnLocker) Error() error {
	err, ok := impl.err.Load().(error)
	if !ok {
		return nil
	}
	return err
}

func (impl *redisDistributedUnLocker) openUnlockFlag() {
	atomic.CompareAndSwapInt32(&impl.status, 0, 1)
}

func (impl *redisDistributedUnLocker) isUnlocked() bool {
	return atomic.LoadInt32(&impl.status) == 1
}

// startRenewal starts a goroutine to renew the lock periodically.
func (impl *redisDistributedUnLocker) startRenewal() {
	go func() {
		renewalErr := fmt.Errorf("renewal err")
		for {
			time.Sleep(time.Duration(impl.expire/lockExpireSplitSection) * time.Second)
			if time.Since(impl.keySetTime) >= time.Duration(impl.expire)*time.Second {
				impl.err.Store(renewalErr)
				return
			}
			if impl.isUnlocked() {
				return
			}
			err := impl.renewal()
			switch err {
			case nil:
				impl.keySetTime = time.Now()
			case ErrLockRenewalFailed:
				impl.openUnlockFlag()
				return
			default:
				renewalErr = err
			}
		}
	}()
}

func (impl *redisDistributedUnLocker) renewal() error {
	result, err := impl.client.Eval(context.Background(), renewalLua, []string{impl.lockKey}, impl.lockUniqueID, impl.expire)
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrLockRenewalFailed
	}
	return nil
}

func (impl *redisDistributedLocker) key(key string) string {
	return fmt.Sprintf("{%s}:%s", impl.hashTag, key)
}

func (impl *redisDistributedLocker) blockKey(key string) string {
	return impl.key(key) + blockQueueKeyword
}

func (impl *redisDistributedLocker) verifyKey(key string) error {
	if strings.HasSuffix(key, blockQueueKeyword) {
		return ErrInvalidKey
	}
	return nil
}
