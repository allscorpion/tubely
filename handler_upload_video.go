package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
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

	outputPath, err := processVideoForFastStart(tempFile.Name());

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to optimize video", err)
		return
	}

	processedFile, err := os.Open(outputPath);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to open optimize video", err)
		return
	}

	defer os.Remove(outputPath);
	defer processedFile.Close();

	fileExtension := getMediaTypeExt(mediaType);
	fileName, err := generateFileName(fileExtension);
	key := path.Join(videoAssetPrefix, fileName);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to generate file name", err)
		return
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        processedFile,
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading file to S3", err)
		return
	}

	videoUrl := fmt.Sprintf("%v/%v", cfg.s3CfDistribution, key);

	video.VideoURL = &videoUrl;

	err = cfg.db.UpdateVideo(video);

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

