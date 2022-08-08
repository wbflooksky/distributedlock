package distributedlock

import (
	"context"
	"errors"
	"time"
)

var (
	ErrLockWaitTimeout = errors.New("the lock was acquired over the maximum time")
	ErrUnlocked        = errors.New("the lock already released")
)

// DistributedLocker is a distributed lock interface
type DistributedLocker interface {
	// Lock locks the given key.
	// - args:
	//   - key: The unique identifier of a lock.
	Lock(ctx context.Context, key string) (UnLocker, error)
	// TryLock try to acquire the lock in the timeout duration, argument and return value are the same as Lock.
	TryLock(ctx context.Context, key string, timeout time.Duration) (UnLocker, error)
}

// UnLocker is the method to unlock the current lock.
type UnLocker func() error
