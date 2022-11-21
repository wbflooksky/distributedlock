package distributedlock

import (
	"errors"
	"time"
)

var (
	ErrLockWaitTimeout    = errors.New("the lock was acquired over the maximum time")
	ErrUnlocked           = errors.New("the lock already released")
	ErrLockExpireTooShort = errors.New("lock expire time is too short")
	ErrInvalidKey         = errors.New("invalid key contain block queue keyword")
	ErrLocked             = errors.New("the lock already occupy")
	ErrLockRenewalFailed  = errors.New("the lock renewal failed")
)

// DistributedLocker is a distributed lock interface
type DistributedLocker interface {
	// Lock sync locks the given key, no wait
	// - args:
	//   - key: The unique identifier of a lock.
	Lock(key string) (UnLocker, error)
	// TryLock try to acquire the lock in the timeout duration, argument and return value are the same as Lock.
	// - args:
	//   - timeout: The time is accurate to the second.
	TryLock(key string, timeout time.Duration) (UnLocker, error)
}

type UnLocker interface {
	// Unlock unlocks the lock.
	UnLock() error
	// Error if lock term internal error, return error
	Error() error
}
