package logs

import "time"

// ParseLog parses a raw Scalingo log line into a LogEntry in UTC.
//
// The returned entry is always usable, even when err is non-nil: Raw is
// always set, and a line without a timestamp gets receivedAt, LevelUnknown
// and the error from ParseEnvelope.
func ParseLog(rawLine string, receivedAt time.Time) (LogEntry, error) {
	envelope, err := ParseEnvelope(rawLine)
	if err != nil {
		return LogEntry{Timestamp: receivedAt.UTC(), Level: LevelUnknown, Raw: envelope.Raw}, err
	}

	if envelope.Container == "router" {
		logEntry := ParseRouterLog(envelope)
		logEntry.Timestamp = logEntry.Timestamp.UTC()
		return logEntry, nil
	}

	logEntry := ParseAppLog(envelope)
	logEntry.Timestamp = logEntry.Timestamp.UTC()
	return logEntry, nil
}
