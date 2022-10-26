package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/google/go-github/v48/github"
	"github.com/umpc/go-sortedmap"
	"github.com/umpc/go-sortedmap/desc"
	"gopkg.in/cheggaaa/pb.v1"
)

var (
	once   sync.Once
	client *github.Client
)

func GetClient() *github.Client {
	once.Do(func() {
		client = github.NewClient(nil)
	})
	return client
}

// listForks lists all forks of repo.
func listForks(owner, repo string) []string {
	client := GetClient()
	forks, _, _ := client.Repositories.ListForks(context.Background(), owner, repo, nil)
	res := []string{}
	for _, f := range forks {
		res = append(res, fmt.Sprintf("%s/%s", *f.Owner.Login, repo))
	}
	return res
}

// compareRepo compares a repository.
// FIXME: Fork chains...
func compareRepo(owner, repo, parent string) (int, int) {
	client := GetClient()
	comparison, _, err := client.Repositories.CompareCommits(context.Background(), owner, repo, "master", parent+":"+repo+":master", nil)
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
	actualArgs := strings.Split(os.Args[1], "/")
	forks := listForks(actualArgs[0], actualArgs[1])
	mahead := make(map[string]int)
	mbehind := make(map[string]int)
	sm := sortedmap.New(len(forks), desc.Int)
	bar := pb.StartNew(len(forks))
	for _, fork := range forks {
		bar.Increment()
		forkArgs := strings.Split(fork, "/")
		ahead, behind := compareRepo(forkArgs[0], forkArgs[1], actualArgs[0])
		if ahead > 0 {
			mahead[fork] = ahead
			mbehind[fork] = behind
			sm.Insert(fork, ahead-behind)
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
			fmt.Fprintln(w, fmt.Sprintf("%s\t+%d -%d", key, mahead[key], mbehind[key]))
		}
		w.Flush()
	}
}
