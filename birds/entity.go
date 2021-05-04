package birds

import (
	"fmt"
	"sort"
)

type entity interface {
	getStart() int
	getEnd() int
	render() string
}

// URLEntity represents an url in tweet's entities.
type URLEntity struct {
	Start       int    `json:"start"`
	End         int    `json:"end"`
	ExpandedURL string `json:"expanded_url"`
}

func (e URLEntity) getStart() int {
	return e.Start
}

func (e URLEntity) getEnd() int {
	return e.End
}

func (e URLEntity) render() string {
	return fmt.Sprintf(
		`<a href="%s">%s</a>`,
		e.ExpandedURL,
		e.ExpandedURL,
	)
}

// MentionEntity represents a mention in tweet's entities.
type MentionEntity struct {
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Username string `json:"username"`
}

func (e MentionEntity) getStart() int {
	return e.Start
}

func (e MentionEntity) getEnd() int {
	return e.End
}

func (e MentionEntity) render() string {
	return fmt.Sprintf(
		`<a href="https://twitter.com/%s">@%s</a>`,
		e.Username,
		e.Username,
	)
}

// Entities represent twitter entities of a single tweet.
type Entities struct {
	URLs     []URLEntity     `json:"urls,omitempty"`
	Mentions []MentionEntity `json:"mentions,omitempty"`
}

func (e Entities) sort() []entity {
	result := make([]entity, 0, len(e.URLs)+len(e.Mentions))
	for _, url := range e.URLs {
		result = append(result, url)
	}
	for _, mention := range e.Mentions {
		result = append(result, mention)
	}
	sort.Slice(result, func(i, j int) bool {
		// Reverse sort them.
		return result[i].getStart() > result[j].getStart()
	})
	return result
}
