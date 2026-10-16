package main

import (
	"sync"
	"time"
)

// 全局任务锁：自动任务与计划任务共用同一条流水线（复制→压缩→上传），
// 操作的是同一个本地目录和数据库。两者同时运行会出现
// 「同一文件被并发压缩导致误删」「同一文件被重复上传、编号重复推送 API2」等冲突，
// 因此限制同一时刻只允许一个任务运行。
//
// 同时提供停止信号：任务内的等待（如失败重试前的 20 秒）通过 awaitTaskStop 实现可中断，
// 使点击「停止任务」后能秒级响应，而不必等整轮跑完。

// taskKind 任务类型
type taskKind string

const (
	taskKindAuto  taskKind = "自动任务"
	taskKindSched taskKind = "计划任务"
)

// taskLockState 全局任务锁状态
type taskLockState struct {
	mu        sync.Mutex
	running   bool
	owner     taskKind
	cancel    chan struct{} // 关闭表示请求停止
	cancelled bool          // 是否已请求停止
}

var globalTaskLock = &taskLockState{}

// tryStartTask 尝试获取任务锁
// 获取成功返回 "", true；已有任务在运行则返回占用者名称与 false
func tryStartTask(kind taskKind) (taskKind, bool) {
	globalTaskLock.mu.Lock()
	defer globalTaskLock.mu.Unlock()

	if globalTaskLock.running {
		return globalTaskLock.owner, false
	}

	globalTaskLock.running = true
	globalTaskLock.owner = kind
	globalTaskLock.cancel = make(chan struct{})
	globalTaskLock.cancelled = false
	return "", true
}

// endTask 释放任务锁，仅持有者可释放，避免误放导致锁失效
func endTask(kind taskKind) {
	globalTaskLock.mu.Lock()
	defer globalTaskLock.mu.Unlock()

	if globalTaskLock.running && globalTaskLock.owner == kind {
		globalTaskLock.running = false
		globalTaskLock.owner = ""
		globalTaskLock.cancel = nil
		globalTaskLock.cancelled = false
	}
}

// runningTask 返回当前正在运行的任务名称，无任务时返回空
func runningTask() taskKind {
	globalTaskLock.mu.Lock()
	defer globalTaskLock.mu.Unlock()

	if !globalTaskLock.running {
		return ""
	}
	return globalTaskLock.owner
}

// cancelRunningTask 请求停止当前运行的任务，任务内的可中断等待会立即返回
func cancelRunningTask() {
	globalTaskLock.mu.Lock()
	defer globalTaskLock.mu.Unlock()

	if !globalTaskLock.running {
		return
	}

	globalTaskLock.cancelled = true
	if globalTaskLock.cancel != nil {
		close(globalTaskLock.cancel)
		globalTaskLock.cancel = nil
	}
}

// taskStopRequested 当前任务是否已被请求停止
func taskStopRequested() bool {
	globalTaskLock.mu.Lock()
	defer globalTaskLock.mu.Unlock()

	return globalTaskLock.cancelled
}

// awaitTaskStop 可中断等待：等待 d 后返回 false；期间收到停止请求则立即返回 true
func awaitTaskStop(d time.Duration) bool {
	globalTaskLock.mu.Lock()
	cancel := globalTaskLock.cancel
	cancelled := globalTaskLock.cancelled
	globalTaskLock.mu.Unlock()

	if cancelled {
		return true
	}
	if cancel == nil {
		time.Sleep(d)
		return false
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-cancel:
		return true
	case <-timer.C:
		return false
	}
}
