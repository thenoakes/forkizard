package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/gocolly/colly"
	"github.com/umpc/go-sortedmap"
	"github.com/umpc/go-sortedmap/desc"
	"gopkg.in/cheggaaa/pb.v1"
)

// forkURL returns the URL for network members.
func forkURL(repo string) string {
	return fmt.Sprintf("https://github.com/%s/network/members", repo)
}

// repoURL returns the URL of a repository.
func repoURL(repo string) string {
	return fmt.Sprintf("https://github.com/%s", repo)
}

// listForks lists all forks of repo.
func listForks(repo string) []string {
	c := colly.NewCollector()
	res := []string{}
	c.OnHTML(".repo a", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		// Only append repository links.
		if strings.Count(href, "/") == 2 {
			res = append(res, href)
		}
	})
	c.Visit(forkURL(repo))
	log.Printf("%d forks\n", len(res))
	return res
}

var re = regexp.MustCompile(`(?P<ahead>\d+) commit[s]? ahead(, (?P<behind>\d+) commit[s]? behind)?`)

// compareRepo compares a repository.
// FIXME: Fork chains...
func compareRepo(fork string) (int, int) {
	ahead := -1
	behind := -1
	c := colly.NewCollector()
	c.OnHTML(".flex-auto.d-flex", func(e *colly.HTMLElement) {
		// Only considering forks ahead.
		if strings.Contains(e.Text, "ahead") {
			match := re.FindStringSubmatch(e.Text)
			ahead, _ = strconv.Atoi(match[1])
			behind, _ = strconv.Atoi(match[3])
		}
	})
	c.Visit(repoURL(fork))
	// if ahead > 0 {
	// 	pp.Println(fork, ahead, behind)
	// }
	return ahead, behind
}

type res struct{ 
	fork string
	ahead, behind int 
}

func main() {
	if (len(os.Args) < 2) {
		os.Stderr.WriteString("Usage: forkizard owner/repo\n")
		os.Exit(1)
	}
	forks := listForks(os.Args[1])
	sm := sortedmap.New(len(forks), desc.Int)
	ch := make(chan res, len(forks))
	bar := pb.StartNew(len(forks))

	var wg sync.WaitGroup
	for _, fork := range forks {
		wg.Add(1)
		go func(fork string) {
			defer wg.Done()
			defer bar.Increment()
			ahead, behind := compareRepo(fork)
			ch <- res{fork: fork, ahead: ahead, behind: behind}
		}(fork)
	}

	wg.Wait()
	close(ch)
	bar.FinishPrint("done")

	results := make(map[string]res, len(forks))
	for i := 1; i <= len(forks); i++ {
		result, ok := <-ch
		if !ok {
			bar.FinishPrint("Error")
			return
		} else if result.ahead > 0 {
				sm.Insert(result.fork, result.ahead-result.behind)
				results[result.fork] = result
		}
	}

	iter, err := sm.IterCh()
	if err != nil {
		bar.FinishPrint(err.Error())
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for rec := range iter.Records() {
		key := rec.Key.(string)
		result := results[key]
		fmt.Fprintln(w, fmt.Sprintf("%s\t+%d -%d", key, result.ahead, result.behind))
	}
	w.Flush()
}
