package handler

import (
	"bytes"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type auditResponseCaptureWriter struct {
	gin.ResponseWriter
	limit int
	buf   bytes.Buffer
}

func attachAuditResponseCapture(c *gin.Context) (*auditResponseCaptureWriter, func()) {
	if c == nil || c.Writer == nil {
		return nil, func() {}
	}
	original := c.Writer
	writer := &auditResponseCaptureWriter{
		ResponseWriter: original,
		limit:          service.ContentModerationLocalAuditResponseCaptureLimitBytes,
	}
	c.Writer = writer
	return writer, func() {
		c.Writer = original
	}
}

func (w *auditResponseCaptureWriter) Write(b []byte) (int, error) {
	w.captureBytes(b)
	return w.ResponseWriter.Write(b)
}

func (w *auditResponseCaptureWriter) WriteString(s string) (int, error) {
	w.captureString(s)
	return w.ResponseWriter.WriteString(s)
}

func (w *auditResponseCaptureWriter) Bytes() []byte {
	if w == nil || w.buf.Len() == 0 {
		return nil
	}
	return append([]byte(nil), w.buf.Bytes()...)
}

func (w *auditResponseCaptureWriter) captureBytes(b []byte) {
	if w == nil || w.limit <= 0 || len(b) == 0 || w.buf.Len() >= w.limit {
		return
	}
	remaining := w.limit - w.buf.Len()
	if len(b) > remaining {
		_, _ = w.buf.Write(b[:remaining])
		return
	}
	_, _ = w.buf.Write(b)
}

func (w *auditResponseCaptureWriter) captureString(s string) {
	if w == nil || w.limit <= 0 || s == "" || w.buf.Len() >= w.limit {
		return
	}
	remaining := w.limit - w.buf.Len()
	if len(s) > remaining {
		_, _ = w.buf.WriteString(s[:remaining])
		return
	}
	_, _ = w.buf.WriteString(s)
}
