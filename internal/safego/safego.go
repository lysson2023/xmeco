package safego

import (
	"context"
	"log/slog"
)

// Go 启动一个带 panic 恢复的 goroutine。如果发生 panic，记录错误日志并使 goroutine
// 干净退出，不会导致整个进程崩溃。
func Go(name string, ctx context.Context, fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("goroutine panic 已恢复", "name", name, "panic", r)
			}
		}()
		fn(ctx)
	}()
}
