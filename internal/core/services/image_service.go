package services

import (
	"context"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
	"github.com/efangly/smtrack-authentication/internal/core/ports"
)

type imageService struct {
	upload ports.FileUploadPort
}

func NewImageService(upload ports.FileUploadPort) ports.ImageService {
	return &imageService{upload: upload}
}

func (s *imageService) Get(ctx context.Context, path string, name string) (*domain.File, error) {
	return s.upload.Download(ctx, path, name)
}
