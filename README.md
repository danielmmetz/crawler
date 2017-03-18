# Crawler -- a concurrent webcrawler written in Go

## Description
Given a set of seed URLs, `crawler` will attempt to crawl those URLs and their children pages recursively, limiting itself to URLs whose domains are amongst those of the seeds.

Could be useful for `sitemap.xml` generation.

## Usage
```
make
./crawler <seed-url> <seed-url> ... <seed-url>
```

## TODOs
- [ ] enable angular support via `flag`
- [ ] if enabled, make dual requests
- [ ] use more helper functions

