package app

type RecoverableProxy interface {
	ProxyController
	Recover() error
}

// Recovery must run even when the business configuration is unreadable.
// The executable acquires the per-user instance lock before calling this.
func NewRecoveredSession(remote Remote, storage Storage, proxy RecoverableProxy) (*Session, error) {
	if err := proxy.Recover(); err != nil {
		return nil, err
	}
	session, err := NewPersistentSession(remote, storage)
	if err != nil {
		return nil, err
	}
	session.SetProxyController(proxy)
	return session, nil
}
