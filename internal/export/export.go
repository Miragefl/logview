package export

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/justfun/logview/internal/model"
)

type Format int

const (
	FormatRaw  Format = iota
	FormatJSON
)

type exportEntry struct {
	Time    string `json:"time,omitempty"`
	Level   string `json:"level,omitempty"`
	Thread  string `json:"thread,omitempty"`
	TraceID string `json:"traceId,omitempty"`
	Logger  string `json:"logger,omitempty"`
	Message string `json:"message,omitempty"`
	Source  string `json:"source,omitempty"`
}

func ToFile(lines []*model.ParsedLine, path string, format Format) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create export file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	count := 0

	for _, line := range lines {
		switch format {
		case FormatRaw:
			fmt.Fprintln(w, line.Raw.Text)
		case FormatJSON:
			entry := exportEntry{
				Time:    line.Get(model.FieldTime),
				Level:   line.Level,
				Thread:  line.Thread,
				TraceID: line.TraceID,
				Logger:  line.Logger,
				Message: line.Message,
				Source:  line.Raw.Source,
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintln(w, string(data))
		}
		count++
	}

	if err := w.Flush(); err != nil {
		return count, fmt.Errorf("flush export: %w", err)
	}
	return count, nil
}