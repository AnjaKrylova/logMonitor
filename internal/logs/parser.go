package logs

import "time"

func ParseLog(rawLine string, receivedAt time.Time) LogEntry {
	envelope, err := ParseEnvelope(rawLine)
	if err != nil {
		return LogEntry{Timestamp: receivedAt.UTC(), Level: LevelUnknown, Raw: envelope.Raw}
	}
	if envelope.Container == "router" {
		logEntry := ParseRouterLog(envelope)
		logEntry.Timestamp = logEntry.Timestamp.UTC()
		return logEntry
	}
	logEntry := ParseAppLog(envelope)
	logEntry.Timestamp = logEntry.Timestamp.UTC()
	return logEntry
}
