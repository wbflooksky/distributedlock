package distributedlock

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v9"
)

var (
	testRedisClient            *redis.ClusterClient
	testRedisDistributedLocker DistributedLocker
)

func setup() error {
	fmt.Println("setup")
	testRedisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{
			"127.0.0.1:6379",
		},
		Password: "123456",
	})
	_, err := testRedisClient.Ping(context.Background()).Result()
	if err != nil {
		return err
	}

	distributedLock, err := NewDistributedLockWithRedis("test", time.Minute, testRedisClient)
	if err != nil {
		return err
	}
	testRedisDistributedLocker = distributedLock
	return nil
}

func teardown() {
	if err := testRedisClient.Close(); err != nil {
		fmt.Println("close redis client err", err)
	}
	fmt.Println("teardown")
}

func Test_Locker(t *testing.T) {
	key := "test"
	var (
		testCount                   int32 = 0
		testConcurrentSecurityCount int32 = 0
	)
	var wg sync.WaitGroup
	const execCount int = 100
	wg.Add(execCount)
	concurrentTest := func() {
		defer wg.Done()
		unLocker, err := testRedisDistributedLocker.TryLock(key, time.Minute)
		if err != nil {
			t.Errorf("Lock key: %s, error: %s", key, err)
		}
		atomic.AddInt32(&testConcurrentSecurityCount, 1)
		testCount++
		if err := unLocker.UnLock(); err != nil {
			t.Errorf("UnLock key: %s, error: %s", key, err)
		}
	}
	for i := 0; i < execCount; i++ {
		go concurrentTest()
	}
	wg.Wait()
	if testConcurrentSecurityCount != int32(execCount) || testCount != int32(execCount) {
		t.Fatalf("want testConcurrentSecurityCount = %v, testCount = %v got testConcurrentSecurityCount = %v, testCount = %v", execCount, execCount, testConcurrentSecurityCount, testCount)
	}
}

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Println("setup failed", err)
		os.Exit(1)
	}
	flag.Parse()
	code := m.Run()
	teardown()
	os.Exit(code)
}
