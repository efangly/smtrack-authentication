package ports

// MessagingPort defines the driven port for message queue operations
type MessagingPort interface {
	SendToDevice(queue string, payload any) error
	SendToLegacy(queue string, payload any) error
}
