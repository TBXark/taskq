package taskq

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// 测试任务正常执行
func TestTaskSuccess(t *testing.T) {
	q := NewQueue[string](0)
	defer q.Close()

	taskID, ch := q.SubmitTask(func() (*string, error) {
		res := "Success"
		return &res, nil
	}, 0, 1*time.Second)

	res := <-ch
	if res.TaskID != taskID {
		t.Errorf("Expected taskID %d, got %d", taskID, res.TaskID)
	}
	if res.Err != nil {
		t.Errorf("Expected error to be nil, got %v", res.Err)
	}
	if *res.Data != "Success" {
		t.Errorf("Expected data 'Success', got %v", res.Data)
	}
}

// 测试任务重试机制
func TestTaskRetry(t *testing.T) {
	var mu sync.Mutex
	var count int
	q := NewQueue[string](0)
	defer q.Close()

	taskID, ch := q.SubmitTask(func() (*string, error) {
		mu.Lock()
		defer mu.Unlock()
		count++
		if count < 2 {
			return nil, errors.New("task failed")
		}
		res := "Success after retry"
		return &res, nil
	}, 2, 1*time.Second)

	res := <-ch
	if res.TaskID != taskID {
		t.Errorf("Expected taskID %d, got %d", taskID, res.TaskID)
	}
	if res.Err != nil {
		t.Errorf("Expected error to be nil, got %v", res.Err)
	}
	if *res.Data != "Success after retry" {
		t.Errorf("Expected data 'Success after retry', got %v", res.Data)
	}
}

// 测试任务超时机制
func TestTaskTimeout(t *testing.T) {
	q := NewQueue[string](0)
	defer q.Close()

	taskID, ch := q.SubmitTask(func() (*string, error) {
		time.Sleep(2 * time.Second)
		res := "Task after sleep"
		return &res, nil
	}, 0, 1*time.Second)

	res := <-ch
	if res.TaskID != taskID {
		t.Errorf("Expected taskID %d, got %d", taskID, res.TaskID)
	}
	if res.Err == nil {
		t.Error("Expected error due to timeout, got nil")
	}
	if res.Data != nil {
		t.Errorf("Expected data to be nil, got %v", res.Data)
	}
}

// 测试队列关闭操作
func TestQueueClose(t *testing.T) {
	q := NewQueue[string](1)

	// 提交一个任务
	taskID1, ch1 := q.SubmitTask(func() (*string, error) {
		res := "Task 1"
		return &res, nil
	}, 0, 1*time.Second)

	// 等待第一个任务完成
	res1 := <-ch1
	if res1.TaskID != taskID1 {
		t.Errorf("Expected taskID %d, got %d", taskID1, res1.TaskID)
	}
	if res1.Err != nil {
		t.Errorf("Expected error to be nil, got %v", res1.Err)
	}
	if *res1.Data != "Task 1" {
		t.Errorf("Expected data 'Task 1', got %v", res1.Data)
	}

	// 关闭队列
	q.Close()

	// 提交另一个任务，应该被忽略
	taskID2, ch2 := q.SubmitTask(func() (*string, error) {
		res := "Task 2"
		return &res, nil
	}, 0, 1*time.Second)

	// 确保 taskID2 为 0
	if taskID2 != 0 {
		t.Errorf("Expected taskID2 0, got %d", taskID2)
	}

	// 确保通道已关闭
	if _, ok := <-ch2; ok {
		t.Error("Expected ch2 to be closed")
	}
}

func TestChannelClosePanic(t *testing.T) {
	queue := NewQueue[int](1)
	
	// 提交第一个任务并获取结果通道
	taskID1, ch1 := queue.SubmitTask(func() (*int, error) {
		result := 42
		return &result, nil
	}, 0, 2*time.Second)

	if taskID1 == 0 {
		t.Error("Expected non-zero taskID for first task")
	}

	// 等待一小段时间确保任务被处理
	time.Sleep(50 * time.Millisecond)

	// 关闭队列，此时任务可能已完成，也可能被中断
	queue.Close()

	// 验证第一个任务的结果
	select {
	case result, ok := <-ch1:
		if !ok {
			// 通道关闭是可接受的，因为队列已关闭
			t.Log("Channel closed due to queue shutdown")
		} else {
			// 如果任务完成了，验证结果
			if result.Err != nil {
				t.Logf("Task completed with error: %v", result.Err)
			} else if result.Data != nil && *result.Data == 42 {
				t.Log("Task completed successfully")
			}
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for first task channel to be closed or receive result")
	}

	// 尝试提交第二个任务，应该返回 taskID 为 0 且通道应该立即关闭
	taskID2, ch2 := queue.SubmitTask(func() (*int, error) {
		return nil, errors.New("test error")
	}, 1, 1*time.Second)

	if taskID2 != 0 {
		t.Error("Expected zero taskID for task submitted after Close")
	}

	// 验证第二个通道是否已关闭
	select {
	case _, ok := <-ch2:
		if ok {
			t.Error("Expected second channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for second channel to close")
	}
}

func TestResourceLeak(t *testing.T) {
	queue := NewQueue[int](1)
	taskCount := 10
	completedTasks := make(chan struct{}, taskCount)
	startTime := time.Now()
	var executionTime time.Duration
	
	// 提交多个任务并跟踪它们的完成情况
	for i := 0; i < taskCount; i++ {
		i := i // 捕获循环变量
		taskID, ch := queue.SubmitTask(func() (*int, error) {
			time.Sleep(100 * time.Millisecond)
			result := i
			return &result, nil
		}, 0, 1*time.Second)

		if taskID == 0 {
			t.Errorf("Task %d: Expected non-zero taskID", i)
		}

		// 在 goroutine 中等待任务完成
		go func() {
			result := <-ch
			if result.Err != nil {
				t.Errorf("Task %d: Unexpected error: %v", i, result.Err)
			}
			if result.Data == nil || *result.Data != i {
				t.Errorf("Task %d: Expected result %d, got %v", i, i, result.Data)
			}
			completedTasks <- struct{}{}
		}()
	}

	// 等待所有任务完成或超时
	timeout := time.After(3 * time.Second)
	completedCount := 0
	success := true
	for completedCount < taskCount {
		select {
		case <-completedTasks:
			completedCount++
		case <-timeout:
			t.Errorf("Timeout waiting for tasks to complete: only %d/%d completed", completedCount, taskCount)
			success = false
			goto cleanup
		}
	}

	if success {
		// 验证总执行时间是否合理
		executionTime = time.Since(startTime)
		if executionTime > 2*time.Second {
			t.Errorf("Tasks took too long to complete: %v", executionTime)
		}
	}

cleanup:
	// 关闭队列
	queue.Close()

	// 等待一段时间确保资源被正确释放
	time.Sleep(500 * time.Millisecond)
}
