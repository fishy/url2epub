package birds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var client http.Client

// Session manages a twitter application-only authentication sesison.
type Session struct {
	UserAgent string
	Bearer    string
}

// Do executes http request with the bearer token in this session.
func (s *Session) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", s.UserAgent)
	req.Header.Set("Authorization", "Bearer "+s.Bearer)
	return client.Do(req)
}

// Tweet represents a single tweet.
type Tweet struct {
	Text     string      `json:"text"`
	ID       json.Number `json:"id"`
	AuthorID json.Number `json:"author_id"`

	CreatedAt       string      `json:"created_at"`
	ConversationID  json.Number `json:"conversation_id,omitempty"`
	InReplyToUserID json.Number `json:"in_reply_to_user_id,omitempty"`

	Entities Entities `json:"entities,omitempty"`

	Attachments struct {
		MediaKeys []string `json:"media_keys,omitempty"`
	} `json:"attachments,omitempty"`

	ReferencedTweets []struct {
		Type string      `json:"type"`
		ID   json.Number `json:"id"`
	} `json:"referenced_tweets,omitempty"`
}

// Media represents a twitter media (e.g. photo)
type Media struct {
	Key    string `json:"media_key"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Type   string `json:"type"`
	URL    string `json:"url"`
}

// Photos defines the map from media key to the photo media.
type Photos = map[string]*Media

// User represents a twitter user.
type User struct {
	ID       json.Number `json:"id"`
	Username string      `json:"username"`
}

// Users defines the map from id to the user.
type Users = map[string]*User

const (
	tweetTemplate = `https://api.twitter.com/2/tweets/%s?`

	conversationURL = `https://api.twitter.com/2/tweets/search/recent?`
)

// Includes represends the included tweets, users, media.
type Includes struct {
	Tweets []*Tweet `json:"tweets"`
	Users  []*User  `json:"users"`
	Media  []*Media `json:"media,omitempty"`
}

func (i *Includes) merge(n Includes) {
	i.Tweets = append(i.Tweets, n.Tweets...)
	i.Users = append(i.Users, n.Users...)
	i.Media = append(i.Media, n.Media...)
}

// Meta represents the metadata portion of a twitter api response.
type Meta struct {
	NextToken string `json:"next_token,omitempty"`
}

type queryResult struct {
	Data     []*Tweet `json:"data"`
	Includes Includes `json:"includes"`
	Meta     Meta     `json:"meta"`
}

func (r *queryResult) merge(n *queryResult) {
	r.Data = append(r.Data, n.Data...)
	r.Includes.merge(n.Includes)
}

func (s *Session) singleQuery(ctx context.Context, orig *tweet, nextToken string) (*queryResult, error) {
	queries := url.Values{
		"query": []string{strings.Join([]string{
			"conversation_id:" + orig.Data.ConversationID.String(),
			"from:" + orig.Data.AuthorID.String(),
			"to:" + orig.Data.AuthorID.String(),
		}, " ")},
		"max_results": []string{"100"},
	}
	if nextToken != "" {
		queries.Set("next_token", nextToken)
	}
	req, err := http.NewRequest(
		http.MethodGet,
		conversationURL+AddTwitterQuery(queries).Encode(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	var result queryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}
	return &result, nil
}

// Thread generates HTML of a twitter thread from a single tweet id.
func (s *Session) Thread(ctx context.Context, id string) (string, error) {
	orig, err := s.singleTweet(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get the original tweet: %w", err)
	}

	var nextToken string
	var result queryResult
	for {
		next, err := s.singleQuery(ctx, orig, nextToken)
		if err != nil {
			return "", err
		}
		result.merge(next)
		nextToken = next.Meta.NextToken
		if nextToken == "" {
			break
		}
	}

	includes := result.Includes
	var slice []*Tweet
	if len(result.Data) == 0 && len(result.Includes.Tweets) == 0 {
		includes = orig.Includes
		slice = []*Tweet{orig.Data}
	} else {
		first, err := s.singleTweet(ctx, orig.Data.ConversationID.String())
		if err != nil {
			return "", fmt.Errorf("failed to get the first tweet %s: %w", orig.Data.ConversationID.String(), err)
		}
		includes.merge(first.Includes)
		slice = append(slice, first.Data)
		slice = append(slice, result.Data...)
		sort.Slice(slice, func(i, j int) bool {
			ii, _ := slice[i].ID.Int64()
			ij, _ := slice[j].ID.Int64()
			return ii < ij
		})
		slice = threader(slice)
	}

	var photos Photos
	if len(includes.Media) > 0 {
		photos = make(Photos)
		for _, m := range includes.Media {
			if m.Type != "photo" {
				continue
			}
			photos[m.Key] = m
		}
	}

	var author string
	title := "A twitter thread"
	for _, user := range includes.Users {
		if user.ID.String() == slice[0].AuthorID.String() {
			author = user.Username
			title += " by @" + author
			break
		}
	}

	var sb strings.Builder
	sb.WriteString("<html>\n<head>\n<title>")
	sb.WriteString(title)
	sb.WriteString("</title>\n")
	// begin meta
	sb.WriteString(`<meta itemprop="generated-by: https://pkg.go.dev/go.yhsif.com/url2epub/birds#Session.Thread"/>`)
	sb.WriteString("\n")
	sb.WriteString(`<meta itemprop="generated-at: `)
	sb.WriteString(time.Now().Format(time.RFC3339))
	sb.WriteString("\"/>\n")
	sb.WriteString(`<meta itemprop="generated-from: https://twitter.com/`)
	sb.WriteString(author)
	sb.WriteString("foo/status/")
	sb.WriteString(id)
	sb.WriteString("\"/>\n")
	// end meta
	sb.WriteString("</head>\n<body>\n<article>\n<h1>")
	sb.WriteString("A twitter thread ")
	if author != "" {
		sb.WriteString(fmt.Sprintf(
			`by <a href="https://twitter.com/%s">@%s</a> `,
			author,
			author,
		))
	}
	sb.WriteString(fmt.Sprintf(
		`at <a href="https://twitter.com/%s/status/%s">%s</a>`,
		author,
		slice[0].ID,
		slice[0].CreatedAt,
	))
	sb.WriteString("</h1>\n")
	for _, t := range slice {
		sb.WriteString(t.RenderHTML(photos))
	}
	sb.WriteString("</article>\n</body>\n</html>\n")
	return sb.String(), nil
}

// Find the longest thread recursively
func threader(all []*Tweet) []*Tweet {
	switch len(all) {
	case 0, 1:
		return all
	}

	first := all[0]
	firstID := first.ID.String()
	var max []*Tweet
	for i := 1; i < len(all); i++ {
		if len(all)-i < len(max) {
			// Can no longer be a longer thread
			break
		}

		var repliedToFirst bool
		for _, ref := range all[i].ReferencedTweets {
			if ref.Type != "replied_to" {
				continue
			}
			if ref.ID.String() == firstID {
				repliedToFirst = true
				break
			}
		}
		if !repliedToFirst {
			continue
		}

		thread := threader(all[i:])
		if len(thread) > len(max) {
			max = thread
		}
	}
	return append([]*Tweet{first}, max...)
}

type tweet struct {
	Data     *Tweet   `json:"data"`
	Includes Includes `json:"Includes"`
}

func (s *Session) singleTweet(ctx context.Context, id string) (*tweet, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf(tweetTemplate, id)+AddTwitterQuery(nil).Encode(),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	var data tweet
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}
	return &data, nil
}
