package main

import (
	"sync"
	"time"
)

// 通知冷却：同类通知在冷却时间内只发送一次，避免持续故障把 webhook 刷爆。
// 冷却期内被抑制的次数会累积，并在下一次实际发送时带上；
// 故障恢复（上传成功）且期间有抑制记录时清空状态，使下次故障立即通知。

// notifyKindUploadFailed 上传失败通知的类型标识
const notifyKindUploadFailed = "upload_failed"

// notifyCooldown 同类通知的冷却时间（测试中可调整）
var notifyCooldown = 5 * time.Minute

// notifyRecord 某一类通知的冷却状态
type notifyRecord struct {
	lastSent     time.Time // 上次实际发送时间
	suppressed   int       // 冷却期内被抑制的次数
	firstBlocked time.Time // 首次被抑制的时间
}

// notifyCooldownState 按通知类型维护冷却状态
type notifyCooldownState struct {
	mu      sync.Mutex
	records map[string]*notifyRecord
}

var uploadNotifyState = &notifyCooldownState{records: make(map[string]*notifyRecord)}

// allow 判断该类通知现在是否可以发送
// 返回：是否可发送、本次冷却期内被抑制的次数、首次被抑制的时间
func (s *notifyCooldownState) allow(kind string, now time.Time) (bool, int, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[kind]
	if !ok {
		s.records[kind] = &notifyRecord{lastSent: now}
		return true, 0, time.Time{}
	}

	// 超过冷却时间：放行，并把累积的抑制次数一并返回
	if now.Sub(rec.lastSent) >= notifyCooldown {
		suppressed, first := rec.suppressed, rec.firstBlocked
		rec.lastSent = now
		rec.suppressed = 0
		rec.firstBlocked = time.Time{}
		return true, suppressed, first
	}

	// 冷却期内：抑制本次通知
	rec.suppressed++
	if rec.firstBlocked.IsZero() {
		rec.firstBlocked = now
	}
	return false, rec.suppressed, rec.firstBlocked
}

// resetIfPending 故障恢复时清空状态，但仅在期间确实抑制过通知时才清空
// 返回是否执行了清空
func (s *notifyCooldownState) resetIfPending(kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[kind]
	if !ok || rec.suppressed == 0 {
		return false
	}
	delete(s.records, kind)
	return true
}
