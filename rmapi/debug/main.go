package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.yhsif.com/url2epub/logger"
	"go.yhsif.com/url2epub/rmapi"
)

var (
	refreshToken = flag.String(
		"refresh-token",
		"",
		"The refresh token for this client. Exactly one of refresh-token and token is required.",
	)
	token = flag.String(
		"token",
		"",
		"The (usually 8 characters long) token you got from https://my.remarkable.com/connect/desktop. Exactly one of refresh-token and token is required.",
	)
	timeout = flag.Duration(
		"timeout",
		time.Second*10,
		"Timeout for the whole session.",
	)

	// upload action
	upload = flag.String(
		"upload",
		"",
		"Path to the .epub or .pdf file to be uploaded.",
	)
	uploadTitle = flag.String(
		"upload-title",
		"",
		"The title of the uploaded file.",
	)
)

func main() {
	flag.Parse()

	if (*refreshToken != "") == (*token != "") {
		log.Fatal("Exactly one of refresh-token and token flag is required.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var client *rmapi.Client
	if *token != "" {
		var err error
		client, err = rmapi.Register(ctx, rmapi.RegisterArgs{
			Token:       *token,
			Description: "desktop-linux",
		})
		if err != nil {
			log.Fatalf("Failed to register client: %v", err)
		}
		log.Printf("Your refresh token is: %q", client.RefreshToken)
		client.Logger = logger.StdLogger(nil)
	} else {
		client = &rmapi.Client{
			RefreshToken: *refreshToken,
			Logger:       logger.StdLogger(nil),
		}
	}

	if *upload != "" {
		if err := doUpload(ctx, client); err != nil {
			log.Fatalf("Unable to upload: %v", err)
		} else {
			log.Print("Upload succeeded.")
		}
	}
}

func doUpload(ctx context.Context, client *rmapi.Client) error {
	var fileType rmapi.FileType
	switch strings.ToLower(filepath.Ext(*upload)) {
	default:
		return fmt.Errorf("unable to determine file type from %q", *upload)
	case ".epub":
		fileType = rmapi.FileTypeEpub
	case ".pdf":
		fileType = rmapi.FileTypePdf
	}

	f, err := os.Open(*upload)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", *upload, err)
	}
	defer f.Close()
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("failed to create uuid: %w", err)
	}

	title := *uploadTitle
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(*upload), filepath.Ext(*upload))
	}

	log.Printf("Uploading using id %q", id.String())

	return client.Upload(ctx, rmapi.UploadArgs{
		ID:    id.String(),
		Title: title,
		Data:  f,
		Type:  fileType,
	})
}
