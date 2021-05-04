package birds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
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
	searchQuery   = `conversation_id:%s from:%s to:%s`
	tweetTemplate = `https://api.twitter.com/2/tweets/%s?`

	conversationURL = `https://api.twitter.com/2/tweets/search/recent?`
)

// Includes represends the included tweets, users, media.
type Includes struct {
	Tweets []*Tweet `json:"tweets"`
	Users  []*User  `json:"users"`
	Media  []*Media `json:"media,omitempty"`
}

type queryResult struct {
	Data     []*Tweet `json:"data"`
	Includes Includes `json:"Includes"`
}

// Thread generates HTML of a twitter thread from a single tweet id.
func (s *Session) Thread(ctx context.Context, id string) (string, error) {
	orig, err := s.singleTweet(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get the original tweet: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodGet,
		conversationURL+AddTwitterQuery(url.Values{
			"query": []string{strings.Join([]string{
				"conversation_id:" + orig.Data.ConversationID.String(),
				"from:" + orig.Data.AuthorID.String(),
				"to:" + orig.Data.AuthorID.String(),
			}, " ")},
		}).Encode(),
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := s.Do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	var result queryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode json: %w", err)
	}

	includes := result.Includes
	var slice []*Tweet
	if len(result.Data) == 0 && len(result.Includes.Tweets) == 0 {
		includes = orig.Includes
		slice = []*Tweet{orig.Data}
	} else {
		slice = append(slice, result.Data...)
		slice = append(slice, result.Includes.Tweets...)
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
	sb.WriteString("</title>\n</head>\n<body>\n<article>\n<h1>")
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
		if len(all)-i < len(max) {
			// Can no longer be a longer thread
			break
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
		io.Copy(ioutil.Discard, resp.Body)
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
