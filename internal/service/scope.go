package service

import (
	"strings"
)

func insufficientAnswer(res QueryResult) bool {
	if res.Path == PathAsk || res.Path == PathRefused || res.Path == PathUnresolved {
		return false
	}
	if wideningBlockedByDegradation(res.Degraded) {
		return false
	}
	return res.RowCount == 0
}

func wideningBlockedByDegradation(degraded string) bool {
	switch degraded {
	case DegradedInvalidSQL, DegradedExecution, DegradedTimeout:
		return true
	default:
		return false
	}
}

func WidenReply(text string) bool {
	return strings.TrimSpace(text) == "WIDEN"
}

func CanWidenAfterInterpretation(res QueryResult, text string) bool {
	return len(res.UnusedDatabases) > 0 && !wideningBlockedByDegradation(res.Degraded) &&
		WidenReply(text)
}
