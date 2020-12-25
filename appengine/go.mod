module github.com/fishy/url2epub/appengine

go 1.15

replace github.com/fishy/url2epub => ../

require (
	cloud.google.com/go v0.74.0
	cloud.google.com/go/datastore v1.3.0
	github.com/fishy/url2epub v0.0.0-00010101000000-000000000000
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d
)
