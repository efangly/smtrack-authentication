package ports

import (
	"context"

	"github.com/efangly/smtrack-authentication/internal/core/domain"
)

// FileUploadPort defines the driven port for file upload operations
type FileUploadPort interface {
	Upload(ctx context.Context, file *domain.File, path string) (string, error)
	Delete(path string, filename string) error
}
