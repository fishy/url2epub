GO=go
GCLOUD=gcloud
PROJECT=url2epub

.PHONY: deploy

deploy:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) build
	rm appengine
	$(GCLOUD) app deploy --project=$(PROJECT)
	rm -Rf vendor/
