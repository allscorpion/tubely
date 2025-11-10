package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close();

	one_gb := 1 << 30;
	http.MaxBytesReader(w, r.Body, int64(one_gb));

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

	video, err := cfg.db.GetVideo(videoID);

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "access denied", err)
		return
	}

	userFile, header, err := r.FormFile("video")
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

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "only mp4 files are allowed", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create temp file", err)
		return
	}

	defer os.Remove(tempFile.Name());
	defer tempFile.Close();

	_, err = io.Copy(tempFile, userFile);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to copy file", err)
		return
	}

	_, err = tempFile.Seek(0, io.SeekStart);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to offset file", err)
		return
	}

	videoAssetPrefix, err := getVideoAssetPrefix(tempFile.Name());

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to generate prefix", err)
		return
	}

	fileExtension := getMediaTypeExt(mediaType);
	fileName, err := generateFileName(fileExtension);
	fullFileName := path.Join(videoAssetPrefix, fileName);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to generate file name", err)
		return
	}

	_, err = cfg.s3Client.PutObject(r.Context(),  &s3.PutObjectInput{
		Bucket: &cfg.s3Bucket,
		Key: &fullFileName,
		Body: tempFile,
		ContentType: &mediaType,
	});

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to create file", err)
		return
	}

	videoUrl := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, fullFileName);

	video.VideoURL = &videoUrl;

	err = cfg.db.UpdateVideo(video);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
