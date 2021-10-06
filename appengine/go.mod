module go.yhsif.com/url2epub/appengine

go 1.16

replace go.yhsif.com/url2epub => ../

require (
	cloud.google.com/go v0.74.0
	cloud.google.com/go/datastore v1.3.0
	github.com/blendle/zapdriver v1.3.1
	go.uber.org/zap v1.16.0
	go.yhsif.com/url2epub v0.0.0-00010101000000-000000000000
	golang.org/x/image v0.0.0-20201208152932-35266b937fa6
	google.golang.org/appengine/v2 v2.0.0-rc2
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d
)
