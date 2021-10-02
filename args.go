package main

import "context"

type contextKey string

// AddShouldRestartedFailedArgToContext - add an argument to the context
func AddShouldRestartedFailedArgToContext(ctx context.Context, value bool) context.Context {
	if value {
		ctx = context.WithValue(ctx, contextKey("shouldRestartedFailed"), value)
	}
	return ctx
}

// GetShouldRestartedFailedArgFromContext - get an argument from the context
func GetShouldRestartedFailedArgFromContext(ctx context.Context) bool {
	shouldRestartedFailed := false
	if v := ctx.Value(contextKey("shouldRestartedFailed")); v != nil {
		shouldRestartedFailed = v.(bool)
	}
	return shouldRestartedFailed
}
