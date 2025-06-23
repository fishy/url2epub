GO=go
GCLOUD=gcloud

project=url2epub
region=us-central1
cloudrunname=url2epub
image=url2epub

# secret token names on https://console.cloud.google.com/security/secret-manager
telegram_token=telegram-token
mailgun_token=mailgun-token
dropbox_token=dropbox-token

# mailgun related settings
mailgun_domain=mg.fishy.me
mailgun_from=url2epub@fishy.me

# dropbox related settings
dropbox_client_id=rtpgsv6ozwlvxfl

gcloud=$(GCLOUD) --project=$(project)
docker_image=$(region)-docker.pkg.dev/$(project)/$(image)/$(image)
full_git_version=$(shell git rev-parse HEAD)
version_tag=$(shell echo $(full_git_version) | cut -c1-12)

memory_limit_mebibytes=2048
gomemlimit=$(shell expr $(memory_limit_mebibytes) / 5 \* 4)

.PHONY: deploy

deploy:
	go mod vendor
	@echo Building $(docker_image)...
	$(gcloud) builds submit --tag $(docker_image)
	@echo Pushing to Google Cloud Run...
	$(gcloud) run deploy $(cloudrunname) \
		--image=$(docker_image) \
		--platform=managed \
		--allow-unauthenticated \
		--region=$(region) \
		--execution-environment=gen2 \
		--cpu=1 \
		--memory=$(memory_limit_mebibytes)Mi \
		--update-env-vars=GOMEMLIMIT=$(gomemlimit)MiB \
		--update-env-vars=CLOUD_PROJECT_ID=$(project) \
		--update-env-vars=VERSION_TAG=$(version_tag) \
		--update-env-vars=MAILGUN_DOMAIN=$(mailgun_domain) \
		--update-env-vars=MAILGUN_FROM=$(mailgun_from) \
		--update-env-vars=DROPBOX_CLIENT_ID=$(dropbox_client_id) \
		--set-secrets="SECRET_TELEGRAM_TOKEN=$(telegram_token):latest,SECRET_MAILGUN_TOKEN=$(mailgun_token):latest,SECRET_DROPBOX_TOKEN=$(dropbox_token):latest"
	rm -Rf vendor

.PHONY: update-secrets

update-secrets:
	$(gcloud) run services update $(cloudrunname) \
		--set-secrets "SECRET_TELEGRAM_TOKEN=$(telegram_token):latest,SECRET_MAILGUN_TOKEN=$(mailgun_token):latest,SECRET_DROPBOX_TOKEN=$(dropbox_token):latest"
