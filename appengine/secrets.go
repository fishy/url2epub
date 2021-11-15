package main

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	smpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

const (
	tokenID       = "telegram-token"
	twitterBearer = "twitter-bearer"

	nameTemplate = "projects/%s/secrets/%s/versions/latest"
)

func getSecret(ctx context.Context, id string) (string, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return "", err
	}
	req := &smpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf(nameTemplate, getProjectID(), id),
	}
	resp, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", err
	}
	return string(resp.Payload.Data), nil
}
