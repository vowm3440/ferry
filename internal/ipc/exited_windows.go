//go:build windows

package ipc

func (p *Process) HighSeq() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.logHigh
}

func (p *Process) Exited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == nil {
		return false
	}
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}
