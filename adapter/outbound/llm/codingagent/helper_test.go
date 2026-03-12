package codingagent

func (s *stubAgent) currentSessionID() string {
	if len(s.sessionIDs) == 0 {
		return ""
	}
	if s.calls > len(s.sessionIDs) {
		return s.sessionIDs[len(s.sessionIDs)-1]
	}
	return s.sessionIDs[s.calls-1]
}
