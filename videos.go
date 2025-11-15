package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os/exec"
	"path"
)

type videoStream struct {
	Width int `json:"width"`
	Height int `json:"height"`
}

type videoDetails struct {
	Streams []videoStream `json:"streams"`
}

func calcAspectRatio(width int, height int) string {
    if height == 0 {
        return "other"
    }
    aspectRatio := float64(width) / float64(height)
    const epsilon = 0.03

    if math.Abs(aspectRatio-16.0/9.0) <= epsilon {
        return "16:9"
    }

    if math.Abs(aspectRatio-9.0/16.0) <= epsilon {
        return "9:16"
    }

    return "other"
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var buffer bytes.Buffer
	cmd.Stdout = &buffer;
	err := cmd.Run()


	if err != nil {
		return "", err;
	}

	var result videoDetails;
	
	if err := json.Unmarshal(buffer.Bytes(), &result); err != nil {
		return "", err;
	}
	
	firstStream := result.Streams[0];

	return calcAspectRatio(firstStream.Width, firstStream.Height), nil;
}

func getVideoAssetPrefix(filePath string) (string, error) {
	aspectRatio, err := getVideoAspectRatio(filePath);

	if err != nil {
		return "", err;
	}

	switch aspectRatio {
	case "16:9":
		return "landscape", nil;
	case "9:16":
		return "portrait", nil;
	default:
		return "other", nil;
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	dir := path.Dir(filePath)
    base := path.Base(filePath)
    outputPath := path.Join(dir, base[:len(base)-len(path.Ext(base))]+".processing.mp4");

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath);

	err := cmd.Run();

	if err != nil {
		return "", err;
	}

	return outputPath, nil;
}
