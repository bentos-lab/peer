package jobqueue

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestManagerRespectsDependencies(t *testing.T) {
	manager := NewManager(1)

	order := make(chan string, 2)
	jobA, err := manager.Enqueue(Job{
		Name: "overview",
		Run: func() error {
			order <- "A"
			return nil
		},
	})
	require.NoError(t, err)

	_, err = manager.Enqueue(Job{
		Name:      "review",
		DependsOn: []string{jobA},
		Run: func() error {
			order <- "B"
			return nil
		},
	})
	require.NoError(t, err)

	select {
	case first := <-order:
		require.Equal(t, "A", first)
	case <-time.After(time.Second):
		t.Fatal("expected first job to run")
	}

	select {
	case second := <-order:
		require.Equal(t, "B", second)
	case <-time.After(time.Second):
		t.Fatal("expected dependent job to run")
	}
}

func TestManagerRespectsMaxWorkers(t *testing.T) {
	manager := NewManager(2)

	var running int32
	var maxObserved int32

	block := make(chan struct{})
	job := func() error {
		current := atomic.AddInt32(&running, 1)
		for {
			prev := atomic.LoadInt32(&maxObserved)
			if current > prev && atomic.CompareAndSwapInt32(&maxObserved, prev, current) {
				break
			}
			if current <= prev {
				break
			}
		}
		<-block
		atomic.AddInt32(&running, -1)
		return nil
	}

	for i := 0; i < 3; i++ {
		_, err := manager.Enqueue(Job{Name: "job", Run: job})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&maxObserved) == 2
	}, time.Second, 10*time.Millisecond)

	close(block)
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&running) == 0
	}, time.Second, 10*time.Millisecond)
}
