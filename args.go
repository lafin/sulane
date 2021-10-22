package main

import "context"

type contextKey string

// AddBoolArgToContext - add a boolean argument to the context
func AddBoolArgToContext(ctx context.Context, key string, value bool) context.Context {
	if value {
		ctx = context.WithValue(ctx, contextKey(key), value)
	}
	return ctx
}

// GetBoolArgFromContext - get a boolean argument from the context
func GetBoolArgFromContext(ctx context.Context, key string) bool {
	value := false
	if v := ctx.Value(contextKey(key)); v != nil {
		value = v.(bool)
	}
	return value
}

// AddStringArgToContext - add a string argument to the context
func AddStringArgToContext(ctx context.Context, key, value string) context.Context {
	if value != "" {
		ctx = context.WithValue(ctx, contextKey(key), value)
	}
	return ctx
}

// GetStringArgFromContext - get a string argument from the context
func GetStringArgFromContext(ctx context.Context, key string) string {
	value := ""
	if v := ctx.Value(contextKey(key)); v != nil {
		value = v.(string)
	}
	return value
}
