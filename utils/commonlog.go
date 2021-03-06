package utils

import (
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	baseTimestamp time.Time
	emptyFieldMap logrus.FieldMap
)

// MyFormatter ...
type MyFormatter struct {
	logrus.TextFormatter
}

// Format ...
func (f *MyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	if f.TextFormatter.DisableLevelTruncation && f.TextFormatter.ForceColors {

		bc, err := f.TextFormatter.Format(entry)
		ticket := string(bc)
		replaced := strings.Replace(ticket, "[20", ""+strings.Repeat(" ", 8-len(entry.Level.String()))+"[20", 1)

		return []byte(replaced), err
	}

	return f.TextFormatter.Format(entry)
}
