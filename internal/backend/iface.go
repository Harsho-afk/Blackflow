package backend

import "net/url"

type Instance interface {
	IsAlive() bool
	SetAlive(bool)
	GetURL() *url.URL
	GetActiveConnections() int64
	Increment()
	Decrement()
}

type Provider interface {
	GetBackends() []Instance
}
