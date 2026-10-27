package analysis

import "context"

type retryNotifierKey struct{}

func WithRetryNotifier(ctx context.Context, notify func()) context.Context {
	return context.WithValue(ctx, retryNotifierKey{}, notify)
}

func NotifyRetry(ctx context.Context) {
	if notify, ok := ctx.Value(retryNotifierKey{}).(func()); ok && notify != nil {
		notify()
	}
}
