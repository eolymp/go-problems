package connector

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/andybalholm/crlf"
	assetpb "github.com/eolymp/go-sdk/eolymp/asset"
)

const objectChunkSize = 5242880

// MultipartUploader appends Uploader interface with multipart upload capabilities.
// It allows to upload large files in chunks, which is useful for files that exceed the size limits of a single upload.
type MultipartUploader struct {
	Uploader
	log Logger
}

func NewMultipartUploader(upload Uploader, log Logger) *MultipartUploader {
	return &MultipartUploader{
		Uploader: upload,
		log:      log,
	}
}

func (p *MultipartUploader) UploadFile(ctx context.Context, path string) (string, error) {
	name := filepath.Base(path)

	hash, err := p.hashFile(path)
	if err != nil {
		return "", err
	}

	key := "sha1:" + hash

	// check if the file is already uploaded
	if out, err := p.Uploader.LookupAsset(ctx, &assetpb.LookupAssetInput{Key: key}); err == nil {
		p.log.Printf("File %v (SHA1: %v) already exists in the storage, using existing link %#v", name, hash, out.GetAssetUrl())
		return out.GetAssetUrl(), nil
	}

	// upload file
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("unable to open file: %w", err)
	}

	defer file.Close()

	start := time.Now()
	chunk := make([]byte, objectChunkSize)
	reader := crlf.NewReader(file)

	upload, err := p.Uploader.StartMultipartUpload(ctx, &assetpb.StartMultipartUploadInput{Name: filepath.Base(path), Type: "text/plain", Keys: []string{key}})
	if err != nil {
		return "", fmt.Errorf("unable to start multipart upload: %w", err)
	}

	var parts []*assetpb.CompleteMultipartUploadInput_Part

	for index := 1; ; index++ {
		size, err := io.ReadFull(reader, chunk)
		if err == io.EOF {
			break
		}

		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return "", fmt.Errorf("unable to read file chunk: %w", err)
		}

		part, err := p.Uploader.UploadPart(ctx, &assetpb.UploadPartInput{
			UploadId:   upload.GetUploadId(),
			PartNumber: uint32(index),
			Data:       chunk[0:size],
		})

		if err != nil {
			return "", fmt.Errorf("unable to upload file chunk (upload #%s): %w", upload.GetUploadId(), err)
		}

		parts = append(parts, &assetpb.CompleteMultipartUploadInput_Part{
			Number: uint32(index),
			Token:  part.GetToken(),
		})
	}

	if len(parts) == 0 {
		return "", nil
	}

	out, err := p.Uploader.CompleteMultipartUpload(ctx, &assetpb.CompleteMultipartUploadInput{
		UploadId: upload.GetUploadId(),
		Parts:    parts,
	})

	if err != nil {
		return "", fmt.Errorf("unable to complete multipart upload: %w", err)
	}

	p.log.Printf("File %v (SHA1: %v) is uploaded to %#v in %v", name, hash, out.GetAssetUrl(), time.Since(start))

	return out.GetAssetUrl(), nil
}

func (p *MultipartUploader) hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer file.Close()

	hash := sha1.New()

	if _, err := io.Copy(hash, crlf.NewReader(file)); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
