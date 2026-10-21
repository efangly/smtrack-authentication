package domain

// File is an uploaded file: its content and the name the client gave it.
type File struct {
	Name string
	// ContentType is detected from Data, never taken from the client.
	ContentType string
	Data        []byte
}
