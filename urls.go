package stats

import (
	"regexp"
	"sort"
	"strings"
)

var urlRegex *regexp.Regexp

func init() {
	urlRegex = regexp.MustCompile(`^(http|https):\/\/|[a-z0-9]+([\-\.]{1}[a-z0-9]+)*\.[a-z]{2,6}(:[0-9]{1,5})?(\/.*)?$`)
}

type TopURL struct {
	URL   string
	Count uint
}

type urls map[string]uint

// NewURLs initializes the urls map.
func NewURLs() urls {
	return make(map[string]uint)
}

// addMessage looks for a url in the message and increments the appropriate
// entry in the urls map.
func (u urls) addMessage(m *Message) {
	words := strings.Split(m.Message, " ")
	for _, w := range words {
		if urlRegex.FindStringSubmatch(w) != nil {
			u[w]++
		}
	}
}

// TopURLs returns the top n most popular urls.
func (u urls) TopURLs(n int) []*TopURL {
	list := make([]*TopURL, 0)

	if len(u) == 0 {
		return list
	}

	for url, count := range u {
		u := &TopURL{URL: url, Count: count}
		list = append(list, u)
	}

	sort.Sort(byCount(list))

	return list[0:n]
}

type byCount []*TopURL

func (a byCount) Len() int           { return len(a) }
func (a byCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byCount) Less(i, j int) bool { return a[i].Count > a[j].Count }