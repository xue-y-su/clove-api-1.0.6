package controller

import (
	"net/url"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const playgroundMetadataSourceLimit = 5

type playgroundMetadataSource struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Domain  string `json:"domain"`
	Snippet string `json:"snippet"`
}

type playgroundMetadataSearch struct {
	Status  string                     `json:"status"`
	Path    string                     `json:"path"`
	Sources []playgroundMetadataSource `json:"sources,omitempty"`
}

type playgroundMetadataEvent struct {
	Type   string                   `json:"type"`
	Search playgroundMetadataSearch `json:"search"`
}

type playgroundMetadataWriter struct {
	gin.ResponseWriter
	outcome service.PlaygroundSearchOutcome
	once    sync.Once
	err     error
}

func newPlaygroundMetadataWriter(writer gin.ResponseWriter, outcome service.PlaygroundSearchOutcome) gin.ResponseWriter {
	if writer == nil || outcome.Status == "" || outcome.Status == service.PlaygroundSearchOutcomeNone {
		return writer
	}
	return &playgroundMetadataWriter{ResponseWriter: writer, outcome: outcome}
}

func (w *playgroundMetadataWriter) Write(data []byte) (int, error) {
	if w.shouldInject(string(data)) {
		w.once.Do(func() { w.err = w.writeMetadata() })
		if w.err != nil {
			return 0, w.err
		}
	}
	return w.ResponseWriter.Write(data)
}

func (w *playgroundMetadataWriter) WriteString(data string) (int, error) {
	if w.shouldInject(data) {
		w.once.Do(func() { w.err = w.writeMetadata() })
		if w.err != nil {
			return 0, w.err
		}
	}
	return w.ResponseWriter.WriteString(data)
}

func (w *playgroundMetadataWriter) shouldInject(data string) bool {
	if !strings.Contains(strings.ToLower(w.Header().Get("Content-Type")), "text/event-stream") {
		return false
	}
	trimmed := strings.TrimLeft(data, " \t\r\n")
	return strings.HasPrefix(trimmed, "data:") || strings.HasPrefix(trimmed, "event:")
}

func (w *playgroundMetadataWriter) writeMetadata() error {
	event := playgroundMetadataEvent{
		Type: "playground.metadata",
		Search: playgroundMetadataSearch{
			Status: playgroundMetadataStatus(w.outcome.Status),
			Path:   w.outcome.Status,
		},
	}
	for _, source := range w.outcome.Sources {
		if len(event.Search.Sources) >= playgroundMetadataSourceLimit {
			break
		}
		parsed, err := url.Parse(source.URL)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		event.Search.Sources = append(event.Search.Sources, playgroundMetadataSource{
			Title:   truncatePlaygroundMetadata(source.Title, 200),
			URL:     source.URL,
			Domain:  strings.ToLower(parsed.Hostname()),
			Snippet: truncatePlaygroundMetadata(source.Content, 320),
		})
	}
	payload, err := common.Marshal(event)
	if err != nil {
		return err
	}
	_, err = w.ResponseWriter.Write(append(append([]byte("data: "), payload...), []byte("\n\n")...))
	return err
}

func playgroundMetadataStatus(outcome string) string {
	if outcome == service.PlaygroundSearchOutcomeTavily || outcome == service.PlaygroundSearchOutcomeNative {
		return "ok"
	}
	return outcome
}

func truncatePlaygroundMetadata(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes)
}
