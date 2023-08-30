package mock

import "github.com/lightninglabs/lightning-terminal/firewalldb"

func NewPrivacyMapDB() *PrivacyMapDB {
	return &PrivacyMapDB{
		r2p: make(map[string]string),
		p2r: make(map[string]string),
	}
}

type PrivacyMapDB struct {
	r2p map[string]string
	p2r map[string]string
}

func (m *PrivacyMapDB) Update(
	f func(tx firewalldb.PrivacyMapTx) error) error {

	return f(m)
}

func (m *PrivacyMapDB) View(
	f func(tx firewalldb.PrivacyMapTx) error) error {

	return f(m)
}

func (m *PrivacyMapDB) NewPair(real, pseudo string) error {
	m.r2p[real] = pseudo
	m.p2r[pseudo] = real
	return nil
}

func (m *PrivacyMapDB) PseudoToReal(pseudo string) (string, error) {
	r, ok := m.p2r[pseudo]
	if !ok {
		return "", firewalldb.ErrNoSuchKeyFound
	}

	return r, nil
}

func (m *PrivacyMapDB) RealToPseudo(real string) (string, error) {
	p, ok := m.r2p[real]
	if !ok {
		return "", firewalldb.ErrNoSuchKeyFound
	}

	return p, nil
}

var _ firewalldb.PrivacyMapDB = (*PrivacyMapDB)(nil)
