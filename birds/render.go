package birds

import (
	"fmt"
	"strings"
)

// RenderHTML renders the tweet text into HTML.
func (t *Tweet) RenderHTML(photos Photos) string {
	// First, replace all urls and mentions
	entities := t.Entities.sort()
	runes := []rune(t.Text)
	for _, e := range entities {
		replaced := runes[:e.getStart()]
		replaced = append(replaced, []rune(e.render())...)
		replaced = append(replaced, runes[e.getEnd():]...)
		runes = replaced
	}
	text := string(runes)

	// Then, add line breakers and get a whole p tag for the text
	text = strings.ReplaceAll(text, "\n", "<br />\n")
	var sb strings.Builder
	sb.WriteString("<p>")
	sb.WriteString(text)
	sb.WriteString("</p>\n")

	// Finally, add any photos
	for _, key := range t.Attachments.MediaKeys {
		if m, ok := photos[key]; ok {
			sb.WriteString(fmt.Sprintf(
				`<p><img src="%s" height="%d" width="%d" /></p>`,
				m.URL,
				m.Height,
				m.Width,
			))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
