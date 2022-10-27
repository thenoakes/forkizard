package main

import (
	"fmt"
	"strings"

	"github.com/google/go-github/v48/github"
)

type repo struct {
	owner, name, key string
}

func ParseKey(key string) *repo {
	parts := strings.Split(key, "/")
	return &repo{
		key: key,
		owner: parts[0],
		name: parts[1],
	}
}

func FromAPI(gr *github.Repository) *repo {
	owner, name := *gr.Owner.Login, *gr.Name
	return &repo{
		key: fmt.Sprintf("%s/%s", owner, name),
		owner: owner,
		name: name,
	}
}