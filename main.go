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

const authEnv = "GITHUB_ACCESS_TOKEN"

var (
	once   sync.Once
	client *github.Client
)

func GetClient() *github.Client {
	once.Do(func() {
		auth := os.Getenv(authEnv)
		if auth != "" {
			ctx := context.Background()
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: auth},
			)
			tc := oauth2.NewClient(ctx, ts)
			client = github.NewClient(tc)
		} else {
			fmt.Println("Running in unauthenticated mode.")
			fmt.Printf("Set %s to a personal access token to use authenticated API requests.\n", authEnv)
			client = github.NewClient(nil)
		}
	})
	return client
}

// listForks lists all forks of repo.
func listForks(r *repo) []*repo {
	client := GetClient()

	pageNumber := 1
	res := []*repo{}
	for {
		fmt.Printf("Fetching page %d...\n", pageNumber)
		forks, _, err := client.Repositories.ListForks(
			context.Background(),
			r.owner,
			r.name,
			&github.RepositoryListForksOptions{
				ListOptions: github.ListOptions{
					PerPage: 100,
					Page:    pageNumber,
				},
			})

		if err != nil {
			fmt.Println(err.Error())
		}

		for _, f := range forks {
			res = append(res, FromAPI(f))
		}

		if len(forks) == 0 {
			break
		} else {
			pageNumber += 1
		}
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
