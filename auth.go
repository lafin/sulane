package main

import (
	"context"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

// GetClient - get github client instance
func GetClient(ctx context.Context, githubToken string) *github.Client {
	oauth2Token := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	oauth2Client := oauth2.NewClient(ctx, oauth2Token)
	return github.NewClient(oauth2Client)
}
