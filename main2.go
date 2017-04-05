package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/jackdanger/collectlinks"
)

const escapeFragmentSuffix string = "?__escaped_fragment__"

type Config struct {
	approvedHosts   []string
	approvedSchemes []string
}

type CandidateSet struct {
	parentHost   string
	parentScheme string
	candidates   []string
}

type VerifyRequest struct {
	url     string
	success bool
}

type CrawlFlow struct {
	chCandidates     chan CandidateSet
	chVerifyRequest  chan VerifyRequest
	chVerifyResponse chan bool
}

func main() {
	tracker := make(map[string]bool)
	config := initConfig(os.Args[1:], nil)
	crawlFlow := initCrawlFlow()

	// initialization
	if len(os.Args[1:]) == 0 {
		os.Exit(0)
	}
	n := 1
	crawlFlow.chCandidates <- CandidateSet{"", "", os.Args[1:]}

	for n > 0 {
		select {
		case candidateSet := <-crawlFlow.chCandidates:
			for _, c := range candidateSet.candidates {
				c, err := fillCandidate(c, candidateSet.parentHost, candidateSet.parentScheme)
				if err != nil {
					continue
				}
				if _, present := tracker[c]; present {
					continue
				}
				if !isApprovedURL(c, config) {
					continue
				}
				tracker[c] = false
				go crawlFlow.crawl(c)
				n++
			}
			n--
		case vr := <-crawlFlow.chVerifyRequest:
			if !vr.success {
				n--
			} else {
				priorSuccess, present := tracker[vr.url]
				if present && priorSuccess {
					crawlFlow.chVerifyResponse <- false
				} else {
					approved := isApprovedURL(vr.url, config)
					tracker[vr.url] = approved
					crawlFlow.chVerifyResponse <- approved
				}
			}
		}
	}

	fmt.Println("Found the following successful and approved urls:")
	for u, success := range tracker {
		if success {
			fmt.Println(u)
		}
	}
}

func (cf *CrawlFlow) crawl(rawurl string) {
	escapedUrl := rawurl + escapeFragmentSuffix
	res, err := http.Get(escapedUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v while getting %v\n", err, rawurl)
		cf.chVerifyRequest <- VerifyRequest{rawurl, false}
		return
	}
	u := res.Request.URL
	u.RawQuery = ""
	u.Fragment = ""

	cf.chVerifyRequest <- VerifyRequest{u.String(), true}
	if approved := <-cf.chVerifyResponse; !approved {
		cf.chCandidates <- CandidateSet{u.Host, u.Scheme, nil}
		return
	}

	children := collectlinks.All(res.Body)
	res.Body.Close()
	cf.chCandidates <- CandidateSet{u.Host, u.Scheme, children}
}

func fillCandidate(c, host, scheme string) (string, error) {
	URL, err := url.Parse(c)
	if err != nil {
		return "", err
	}
	if URL.Host == "" {
		URL.Host = host
		URL.Scheme = scheme
	}
	return URL.String(), nil
}

func initConfig(rawHosts []string, schemes []string) Config {
	if schemes == nil {
		schemes = append(schemes, "http", "https")
	}

	var hosts []string
	for _, rawHost := range rawHosts {
		if u, err := url.Parse(rawHost); err == nil {
			hosts = append(hosts, u.Host)
		}
	}

	return Config{hosts, schemes}
}

func initCrawlFlow() CrawlFlow {
	return CrawlFlow{make(chan CandidateSet, 1), make(chan VerifyRequest), make(chan bool)}
}

func isApprovedURL(s string, c Config) bool {
	u, err := url.Parse(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v while parsing %v for approval\n", err, u)
		return false
	}

	approvedHost := false
	for _, host := range c.approvedHosts {
		if u.Host == host {
			approvedHost = true
			break
		}
	}
	approvedScheme := false
	for _, scheme := range c.approvedSchemes {
		if u.Scheme == scheme {
			approvedScheme = true
			break
		}
	}
	return approvedHost && approvedScheme
}
