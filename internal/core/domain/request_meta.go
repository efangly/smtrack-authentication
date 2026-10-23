package domain

import "context"

// RequestMeta describes the HTTP request an action came from. The audit trail records it
// (ISO/IEC 27001 A.8.15). It travels in the context so service signatures stay unchanged.
type RequestMeta struct {
	IP        string
	UserAgent string
	RequestID string
}

type requestMetaKey struct{}

// WithRequestMeta returns ctx carrying meta.
func WithRequestMeta(ctx context.Context, meta RequestMeta) context.Context {
	return context.WithValue(ctx, requestMetaKey{}, meta)
}

// RequestMetaFrom returns the request metadata in ctx, or the zero value.
func RequestMetaFrom(ctx context.Context) RequestMeta {
	meta, _ := ctx.Value(requestMetaKey{}).(RequestMeta)
	return meta
}
