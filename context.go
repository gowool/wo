package wo

import "context"

type (
	ctxRequestLoggedKey struct{}
	ctxDebugKey         struct{}
)

func WithDebug(ctx context.Context, debug bool) context.Context {
	return context.WithValue(ctx, ctxDebugKey{}, debug)
}

func Debug(ctx context.Context) bool {
	debug, _ := ctx.Value(ctxDebugKey{}).(bool)
	return debug
}

func WithRequestLogged(ctx context.Context, logged bool) context.Context {
	return context.WithValue(ctx, ctxRequestLoggedKey{}, logged)
}

func RequestLogged(ctx context.Context) bool {
	logged, _ := ctx.Value(ctxRequestLoggedKey{}).(bool)
	return logged
}
