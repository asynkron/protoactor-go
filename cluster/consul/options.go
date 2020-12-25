package consul

import "time"

type Option func(p *Provider)

func WithTTL(ttl time.Duration) Option {
	return func(p *Provider) {
		p.ttl = ttl
	}
}

func WithRefreshTTL(refreshTTL time.Duration) Option {
	return func(p *Provider) {
		p.refreshTTL = refreshTTL
	}
}
