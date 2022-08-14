GO=go
GCLOUD=gcloud

project=url2epub
region=us-central1
cloudrunname=url2epub
image=url2epub

secret_telegram_token=telegram-token
secret_twitter_bearer=twitter-bearer

gcloud=$(GCLOUD) --project=$(project)
docker_image=$(region)-docker.pkg.dev/$(project)/$(image)/$(image)
full_git_version=$(shell git rev-parse HEAD)
version_tag=$(shell echo $(full_git_version) | cut -c1-12)

.PHONY: deploy

deploy:
	go mod vendor
	@echo Building $(docker_image)...
	$(gcloud) builds submit --tag $(docker_image)
	@echo Pushing to Google Cloud Run...
	$(gcloud) run deploy --image $(docker_image) \
		--platform managed \
		--allow-unauthenticated \
		--region $(region) $(cloudrunname) \
		--update-env-vars CLOUD_PROJECT_ID=$(project) \
		--update-env-vars VERSION_TAG=$(version_tag) \
		--set-secrets "SECRET_TELEGRAM_TOKEN=$(secret_telegram_token):latest,SECRET_TWITTER_BEARER=$(secret_twitter_bearer):latest"
	rm -Rf vendor

.PHONY: update-secrets

update-secrets:
	$(gcloud) run services update $(cloudrunname) \
		--set-secrets "SECRET_TELEGRAM_TOKEN=$(secret_telegram_token):latest,SECRET_TWITTER_BEARER=$(secret_twitter_bearer):latest"
