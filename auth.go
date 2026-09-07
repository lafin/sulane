package main

import (
	"context"
	"net/http"
	"time"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

// GetClient - get github client instance
func GetClient(ctx context.Context, githubToken string) *github.Client {
	oauth2Token := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	base := &http.Client{Timeout: 30 * time.Second}
	oauth2Client := oauth2.NewClient(context.WithValue(ctx, oauth2.HTTPClient, base), oauth2Token)
	return github.NewClient(oauth2Client)
}
