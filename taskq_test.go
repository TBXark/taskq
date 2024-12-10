package taskq

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// 测试任务正常执行
func TestTaskSuccess(t *testing.T) {
	q := NewQueue[string]()
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
	q := NewQueue[string]()
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
	q := NewQueue[string]()
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
	q := NewQueue[string]()

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
