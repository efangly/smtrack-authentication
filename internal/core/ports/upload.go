package ports

import "mime/multipart"

// FileUploadPort defines the driven port for file upload operations
type FileUploadPort interface {
	Upload(file *multipart.FileHeader, path string) (string, error)
	Delete(path string, filename string) error
}
