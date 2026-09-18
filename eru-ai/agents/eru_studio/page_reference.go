package eru_studio

import (
	"regexp"
	"strings"
)

// A reference page is one the user points at - "make the grid look like the one
// on invoice_360_detail". The agent can read any page in the workspace, and the
// prompt tells it to; nothing checked that it did.
//
// It does not fail loudly when it skips the lookup. The model lists the pages,
// recognises the name, and writes a plausible grid from memory - then reports
// that it matched the page it never opened. The user is told their page now
// looks like another page, and it does not.

// minReferenceNameLength keeps generic names ("form", "home") from turning an
// ordinary sentence into a required lookup.
const minReferenceNameLength = 6

// PageRef is one entry from the page list: enough to recognise a page the user
// named and to fetch it.
type PageRef struct {
	PageId string
	Name   string
}

// RecordRequest keeps the user's own words, so a claim about them can be checked
// later without threading the prompt through the validator.
func (l *Ledger) RecordRequest(text string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.request = text
}

// RecordListedPages notes what the page list returned, which is how a name the
// user used becomes a page id that can be checked.
func (l *Ledger) RecordListedPages(pages []PageRef) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, page := range pages {
		if page.PageId == "" || page.Name == "" {
			continue
		}
		l.listedPages[page.PageId] = page.Name
	}
}

// RecordFetchedPage marks a page as actually read.
func (l *Ledger) RecordFetchedPage(pageId string) {
	if l == nil || pageId == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fetchedPages[pageId] = true
}

// UnreadReferencedPages names the pages the request points at that the agent
// never opened.
//
// currentPageId is the page being edited: the user naming the page they are
// looking at is not a reference to go and read.
func (l *Ledger) UnreadReferencedPages(currentPageId string) []string {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.request == "" || len(l.listedPages) == 0 {
		return nil
	}
	request := strings.ToLower(l.request)

	var unread []string
	for pageId, name := range l.listedPages {
		if pageId == currentPageId || l.fetchedPages[pageId] {
			continue
		}
		if len(name) < minReferenceNameLength {
			continue
		}
		if !mentions(request, strings.ToLower(name)) {
			continue
		}
		unread = append(unread, name)
	}
	return unread
}

var wordEdge = regexp.MustCompile(`[a-z0-9_]`)

// mentions reports a whole-token occurrence, so "invoice_360" does not match
// inside "invoice_360_detail" and vice versa.
func mentions(text string, name string) bool {
	for at := 0; at < len(text); {
		index := strings.Index(text[at:], name)
		if index < 0 {
			return false
		}
		start := at + index
		end := start + len(name)
		beforeOk := start == 0 || !wordEdge.MatchString(string(text[start-1]))
		afterOk := end == len(text) || !wordEdge.MatchString(string(text[end]))
		if beforeOk && afterOk {
			return true
		}
		at = start + 1
	}
	return false
}
