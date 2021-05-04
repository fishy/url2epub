package birds

import (
	"net/url"
)

const (
	expansionsKey = `expansions`
	expansions    = `author_id,attachments.media_keys,referenced_tweets.id,entities.mentions.username,in_reply_to_user_id`

	tweetFieldsKey = `tweet.fields`
	tweetFields    = `conversation_id,created_at,author_id,entities,in_reply_to_user_id,text`

	mediaFieldsKey = `media.fields`
	mediaFields    = `height,media_key,type,url,width`
)

// AddTwitterQuery adds extra expansion and field queries.
func AddTwitterQuery(query url.Values) url.Values {
	if query == nil {
		query = make(url.Values)
	}
	query.Set(expansionsKey, expansions)
	query.Set(tweetFieldsKey, tweetFields)
	query.Set(mediaFieldsKey, mediaFields)
	return query
}
