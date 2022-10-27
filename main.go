package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"text/tabwriter"

	"github.com/google/go-github/v48/github"
	"github.com/umpc/go-sortedmap"
	"github.com/umpc/go-sortedmap/desc"
	"golang.org/x/oauth2"
	"gopkg.in/cheggaaa/pb.v1"
)

var (
	once   sync.Once
	client *github.Client
)

func GetClient() *github.Client {
	once.Do(func() {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN")},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	})
	return client
}

// listForks lists all forks of repo.
func listForks(r *repo) []*repo {
	client := GetClient()
	forks, _, _ := client.Repositories.ListForks(context.Background(), r.owner, r.name, nil)
	res := []*repo{}
	for _, f := range forks {
		res = append(res, FromAPI(f))
	}
	return res
}

// compareRepo compares a repository.
// FIXME: Fork chains...
func compareRepo(fork *repo, parent *repo) (int, int) {
	client := GetClient()
	comparison, _, err := client.Repositories.CompareCommits(
		context.Background(),
		fork.owner,
		fork.name,
		"master",
		parent.owner+":"+parent.name+":master",
		nil)

	if err != nil {
		fmt.Printf("%s", err.Error())
		return 0, 0
	}

	return *comparison.AheadBy, *comparison.BehindBy
}

func main() {
	if len(os.Args) < 2 {
		os.Stderr.WriteString("Usage: forkizard owner/repo\n")
		os.Exit(1)
	}

	parentRepo := ParseKey(os.Args[1])
	forks := listForks(parentRepo)

	mahead := make(map[string]int)
	mbehind := make(map[string]int)
	sm := sortedmap.New(len(forks), desc.Int)
	bar := pb.StartNew(len(forks))
	for _, fork := range forks {
		bar.Increment()
		ahead, behind := compareRepo(fork, parentRepo)
		if ahead > 0 {
			mahead[fork.key] = ahead
			mbehind[fork.key] = behind
			sm.Insert(fork.key, ahead-behind)
		}
	}
	bar.FinishPrint("done")

	iter, err := sm.IterCh()
	if err != nil {
		bar.FinishPrint(err.Error())
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
		for rec := range iter.Records() {
			key := rec.Key.(string)
			fmt.Fprintf(w, "%s\t+%d -%d\n", key, mahead[key], mbehind[key])
		}
		w.Flush()
	}
}
