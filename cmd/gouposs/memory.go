package main

import (
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// 内存回收辅助。
//
// 背景：Go 的 GC 回收对象后并不会立即把内存页还给操作系统，
// 任务管理器里看到的是进程 Working Set（曾经用过的物理页），
// 所以会出现「涨到几 GB 后只慢慢往下掉十几 MB」的观感。
// 图片处理属于典型的「短时间内大量临时大对象」场景，
// 这里在批量处理过程中定期调用 FreeOSMemory，把空闲页真正还给系统。

const (
	// memoryReleaseInterval 两次强制回收之间的最小间隔
	memoryReleaseInterval = 5 * time.Second
	// memoryReleaseThreshold 堆占用超过该值才值得强制回收（字节）
	memoryReleaseThreshold = 256 * 1024 * 1024
)

var (
	memoryReleaseMu   sync.Mutex
	memoryReleaseLast time.Time
)

// ReleaseMemoryIfNeeded 在堆占用偏高且距上次回收已超过间隔时，
// 把内存归还给操作系统；调用开销可控，可在处理循环中高频调用
func ReleaseMemoryIfNeeded() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	if stats.HeapAlloc < memoryReleaseThreshold {
		return
	}

	memoryReleaseMu.Lock()
	defer memoryReleaseMu.Unlock()

	if time.Since(memoryReleaseLast) < memoryReleaseInterval {
		return
	}
	memoryReleaseLast = time.Now()

	debug.FreeOSMemory()
}

// ReleaseMemory 无条件把内存归还给操作系统，用于任务结束时收尾
func ReleaseMemory() {
	memoryReleaseMu.Lock()
	memoryReleaseLast = time.Now()
	memoryReleaseMu.Unlock()

	debug.FreeOSMemory()
}
