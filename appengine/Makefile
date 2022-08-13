GO=go
GCLOUD=gcloud
p=url2epub

.PHONY: deploy tail

deploy:
	$(GO) mod tidy
	$(GO) mod vendor
	$(GO) test
	$(GCLOUD) app deploy --project=$(p)
	rm -Rf vendor/

tail:
	$(GCLOUD) app logs tail -s default --project=$(p)
