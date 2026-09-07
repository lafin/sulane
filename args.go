package main

import "context"

type contextKey string

const (
	keyVerbose                            contextKey = "verbose"
	keyDry                                contextKey = "dry"
	keyShouldRestartFailed                contextKey = "shouldRestartFailed"
	keyShouldReactivateSuspendedWorkflows contextKey = "shouldReactivateSuspendedWorkflows"
	keySkipArchive                        contextKey = "skipArchive"
	keyLast                               contextKey = "last"
	keyQuery                              contextKey = "query"
	keyShouldAutoApproveIfReviewedBy      contextKey = "shouldAutoApproveIfReviewedBy"
	keyShouldAutoApproveIfCreatedBy       contextKey = "shouldAutoApproveIfCreatedBy"
	keyBot                                contextKey = "bot"
	keyChatID                             contextKey = "chatID"
)

// AddBoolArgToContext - add a boolean argument to the context
func AddBoolArgToContext(ctx context.Context, key contextKey, value bool) context.Context {
	return context.WithValue(ctx, key, value)
}

// GetBoolArgFromContext - get a boolean argument from the context
func GetBoolArgFromContext(ctx context.Context, key contextKey) bool {
	value, _ := ctx.Value(key).(bool)
	return value
}

// AddStringArgToContext - add a string argument to the context
func AddStringArgToContext(ctx context.Context, key contextKey, value string) context.Context {
	return context.WithValue(ctx, key, value)
}

// GetStringArgFromContext - get a string argument from the context
func GetStringArgFromContext(ctx context.Context, key contextKey) string {
	value, _ := ctx.Value(key).(string)
	return value
}

// AddAnyArgToContext - add an argument to the context
func AddAnyArgToContext(ctx context.Context, key contextKey, value any) context.Context {
	return context.WithValue(ctx, key, value)
}

// GetAnyArgFromContext - get an argument from the context
func GetAnyArgFromContext(ctx context.Context, key contextKey) any {
	return ctx.Value(key)
}
