module go.yhsif.com/url2epub/appengine

go 1.16

replace go.yhsif.com/url2epub => ../

require (
	cloud.google.com/go/datastore v1.6.0
	cloud.google.com/go/secretmanager v1.0.0
	github.com/blendle/zapdriver v1.3.1
	go.uber.org/zap v1.19.1
	go.yhsif.com/url2epub v0.0.0-00010101000000-000000000000
	golang.org/x/image v0.0.0-20211028202545-6944b10bf410
	google.golang.org/appengine/v2 v2.0.1
	google.golang.org/genproto v0.0.0-20210924002016-3dee208752a0
)
