package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getMediaTypeExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func generateRandomId() (string, error) {
	randId := make([]byte, 32)
	_, err := rand.Read(randId);

	if err != nil {
		return "", nil;
	}

	id := base64.RawURLEncoding.EncodeToString(randId);

	return id, nil;
}

func generateFileName(fileExtension string) (string, error) {
	id, err := generateRandomId();

	if err != nil {
		return "", err;
	}

	fileName := fmt.Sprintf("%v%v", id, fileExtension);

	return fileName, nil;
}

