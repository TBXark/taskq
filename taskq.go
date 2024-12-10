package taskq

import (
	"errors"
	"sync/atomic"
	"time"
)

type Task[T any] struct {
	ID         int64
	Fn         func() (*T, error)
	Result     chan TaskResult[T]
	MaxRetries int
	RetryCount int
	Timeout    time.Duration
}

type TaskResult[T any] struct {
	TaskID int64
	Data   *T
	Err    error
}

type Queue[T any] struct {
	taskQueue  chan *Task[T]
	retryQueue chan *Task[T]
	quit       chan struct{}
	taskId     atomic.Int64
	close      atomic.Bool
}

func NewQueue[T any](size int) *Queue[T] {
	q := &Queue[T]{
		taskQueue:  make(chan *Task[T], size),
		retryQueue: make(chan *Task[T], size),
		quit:       make(chan struct{}),
	}
	go q.taskProcessor()
	return q
}

func (q *Queue[T]) taskProcessor() {
	retryHandler := func() {
		for {
			select {
			case task, ok := <-q.retryQueue:
				if !ok {
					return
				}
				select {
				case q.taskQueue <- task:
				case <-q.quit:
					return
				}
			case <-q.quit:
				return
			}
		}
	}
	go retryHandler()

	for {
		select {
		case task, ok := <-q.taskQueue:
			if !ok {
				return
			}
			done := make(chan struct{})
			var data *T
			var err error
			go func() {
				defer func() {
					if r := recover(); r != nil {
						err = errors.New("panic occurred")
					}
					close(done)
				}()
				data, err = task.Fn()
			}()
			select {
			case <-done:
			case <-time.After(task.Timeout):
				err = errors.New("task execution timed out")
			case <-q.quit:
				return
			}
			if err != nil && task.RetryCount < task.MaxRetries {
				task.RetryCount++
				select {
				case q.retryQueue <- task:
				case <-q.quit:
					return
				}
				continue
			}
			select {
			case task.Result <- TaskResult[T]{task.ID, data, err}:
				close(task.Result)
			case <-q.quit:
				return
			}
		case <-q.quit:
			return
		}
	}
}

func (q *Queue[T]) SubmitTask(fn func() (*T, error), maxRetries int, timeout time.Duration) (int64, <-chan TaskResult[T]) {
	if q.close.Load() {
		ch := make(chan TaskResult[T], 1)
		close(ch)
		return 0, ch
	}

	taskID := q.taskId.Add(1)
	resultChan := make(chan TaskResult[T], 1)
	task := &Task[T]{
		ID:         taskID,
		Fn:         fn,
		Result:     resultChan,
		MaxRetries: maxRetries,
		RetryCount: 0,
		Timeout:    timeout,
	}

	select {
	case q.taskQueue <- task:
		return taskID, resultChan
	default:
		if q.close.Load() {
			close(resultChan)
			return 0, resultChan
		}
		q.taskQueue <- task
		return taskID, resultChan
	}
}

func (q *Queue[T]) Close() {
	if !q.close.CompareAndSwap(false, true) {
		return
	}
	close(q.quit)
	close(q.taskQueue)
	close(q.retryQueue)
}
