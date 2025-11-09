package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory);

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to parse form data", err)
		return
	}

	userFile, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer userFile.Close()
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"));

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "failed to parse content type", err)
		return
	}

	allowedMediaTypes := map[string]struct{}{
		"image/jpeg": {},
		"image/png": {},
	};

	if _, exists := allowedMediaTypes[mediaType]; !exists {
		respondWithError(w, http.StatusBadRequest, "invalid file type", err)
		return
	} 


	video, err := cfg.db.GetVideo(videoID);

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "access denied", err)
		return
	}

	fileExtension := getMediaTypeExt(mediaType);
	randId := make([]byte, 32)
	_, err = rand.Read(randId);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create id", err)
		return
	}

	rawUrl := base64.RawURLEncoding.EncodeToString(randId);
	fileName := fmt.Sprintf("%v.%v", rawUrl, fileExtension);
	fullFilePath := filepath.Join(cfg.assetsRoot, fileName);

	assetFile, err := os.Create(fullFilePath);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "internal server error", err)
		return
	}

	defer assetFile.Close();

	_, err = io.Copy(assetFile, userFile);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "internal server error", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%v/assets/%v", cfg.port, fileName);
	video.ThumbnailURL = &url;

	err = cfg.db.UpdateVideo(video);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
