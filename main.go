package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/jackdanger/collectlinks"
)

const ESCAPE_FRAGMENT_SUFFIX = "?__escaped_fragment__"

var APPROVED_HOSTS []string // to be configured
var APPROVED_SCHEMES = [...]string{"http", "https"}

var lock *sync.RWMutex
var visited map[string]bool // true if approved
var chCandidates chan string
var chFinished chan int

func main() {
	start := time.Now()
	defer func() {
		fmt.Printf("Completed in %v", time.Since(start))
	}()

	lock = &sync.RWMutex{}
	visited = make(map[string]bool)
	chCandidates = make(chan string, 256)
	chFinished = make(chan int, 256)

	seedURLs := os.Args[1:]
	configApprovedHosts(seedURLs)
	for _, u := range seedURLs {
		go crawl(u, chCandidates, chFinished)
	}

	n := len(os.Args[1:])
	for n > 0 {
		select {
		case u := <-chCandidates:
			go crawl(u, chCandidates, chFinished)
		case k := <-chFinished:
			n += k - 1
		}
	}

	fmt.Println("Found the following successful and approved urls:")
	for u, b := range visited {
		if b {
			fmt.Println(u)
		}
	}
}

func configApprovedHosts(seedURLs []string) {
	for _, rawurl := range seedURLs {
		if u, err := url.Parse(rawurl); err == nil {
			APPROVED_HOSTS = append(APPROVED_HOSTS, u.Host)
		}
	}
}

func crawl(rawurl string, chCandidates chan string, chFinished chan int) {
	rawurl += ESCAPE_FRAGMENT_SUFFIX
	res, err := http.Get(rawurl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v while getting %v\n", err, rawurl)
		safeMarkVisited(rawurl, false)
		chFinished <- 0
		return
	}
	u := res.Request.URL
	u.RawQuery = ""
	u.Fragment = ""

	if !approvedURL(u.String()) {
		safeMarkVisited(u.String(), false)
		chFinished <- 0
		return
	}

	children := collectlinks.All(res.Body)
	res.Body.Close()
	var approvedChildren []string

	lock.RLock()
	for _, child := range children {
		childURL, err := url.Parse(child)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v while parsing child %v\n", err, child)
			continue
		}
		if childURL.Host == "" {
			childURL.Host = u.Host
			childURL.Scheme = u.Scheme
		}
		if !approvedURL(childURL.String()) {
			continue
		}
		if _, b := visited[childURL.String()]; b {
			continue
		}
		approvedChildren = append(approvedChildren, childURL.String())
	}
	lock.RUnlock()

	chFinished <- len(approvedChildren)
	lock.Lock()
	visited[u.String()] = true
	for _, child := range approvedChildren {
		visited[child] = false
	}
	lock.Unlock()
	for _, child := range approvedChildren {
		chCandidates <- child
	}
}

func safeMarkVisited(k string, b bool) {
	lock.Lock()
	defer lock.Unlock()
	visited[k] = b
}

func approvedURL(u string) bool {
	url, err := url.Parse(u)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v while parsing %v for approval\n", err, u)
		return false
	}
	approvedHost := false
	for _, host := range APPROVED_HOSTS {
		if url.Host == host {
			approvedHost = true
			break
		}
	}
	approvedScheme := false
	for _, scheme := range APPROVED_SCHEMES {
		if url.Scheme == scheme {
			approvedScheme = true
			break
		}
	}
	return approvedHost && approvedScheme
}
