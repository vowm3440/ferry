package app

func (s *Session) SetEmitter(emit func(string, any)) {
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	s.emitter = emit
}

func (s *Session) emit(name string, payload any) {
	s.emitMu.RLock()
	emit := s.emitter
	s.emitMu.RUnlock()
	if emit != nil {
		emit(name, payload)
	}
}

type subProgress struct {
	ID    string `json:"id"`
	Phase string `json:"phase"`
	Error string `json:"error,omitempty"`
}

func (s *Subs) progress(id, phase, message string) {
	s.session.emit("subs:progress", subProgress{ID: id, Phase: phase, Error: message})
}
