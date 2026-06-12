package utils

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"cloud.google.com/go/storage"
)

func DeleteImageFromStorage(ctx context.Context, bucket *storage.BucketHandle, imageURL string) error {
	parts := strings.Split(imageURL, "/o/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid image URL format")
	}
	pathWithQuery := strings.Split(parts[1], "?")[0]
	path, _ := url.QueryUnescape(pathWithQuery)
	return bucket.Object(path).Delete(ctx)
}
