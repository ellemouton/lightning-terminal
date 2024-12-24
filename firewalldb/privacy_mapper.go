package firewalldb

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/lightninglabs/lightning-terminal/db"
	"github.com/lightninglabs/lightning-terminal/session"
)

const txidStringLen = 64

// NewPrivacyMapDB is a function type that takes a group ID and uses it to
// construct a new PrivacyMapDB.
type PrivMapDBCreator interface {
	PrivacyDB(groupID session.ID) PrivacyMapDB
}

// PrivacyMapDB provides an Update and View method that will allow the caller
// to perform atomic read and write transactions defined by PrivacyMapTx on the
// underlying DB.
type PrivacyMapDB interface {
	// Update opens a database read/write transaction and executes the
	// function f with the transaction passed as a parameter. After f exits,
	// if f did not error, the transaction is committed. Otherwise, if f did
	// error, the transaction is rolled back. If the rollback fails, the
	// original error returned by f is still returned. If the commit fails,
	// the commit error is returned.
	Update(ctx context.Context, f func(tx PrivacyMapTx) error) error

	// View opens a database read transaction and executes the function f
	// with the transaction passed as a parameter. After f exits, the
	// transaction is rolled back. If f errors, its error is returned, not a
	// rollback error (if any occur).
	View(ctx context.Context, f func(tx PrivacyMapTx) error) error
}

// PrivacyMapTx represents a db that can be used to create, store and fetch
// real-pseudo pairs.
type PrivacyMapTx interface {
	db.Tx

	// NewPair persists a new real-pseudo pair.
	NewPair(real, pseudo string) error

	// PseudoToReal returns the real value associated with the given pseudo
	// value. If no such pair is found, then ErrNoSuchKeyFound is returned.
	PseudoToReal(pseudo string) (string, error)

	// RealToPseudo returns the pseudo value associated with the given real
	// value. If no such pair is found, then ErrNoSuchKeyFound is returned.
	RealToPseudo(real string) (string, error)

	// FetchAllPairs loads and returns the real-to-pseudo pairs in the form
	// of a PrivacyMapPairs struct.
	FetchAllPairs() (*PrivacyMapPairs, error)
}

type txCreator interface {
	beginTx(ctx context.Context, writable bool) (PrivacyMapTx, error)
}

// privacyMapDB is an implementation of PrivacyMapDB.
type privacyMapDB struct {
	db txCreator
}

// Update opens a database read/write transaction and executes the function f
// with the transaction passed as a parameter. After f exits, if f did not
// error, the transaction is committed. Otherwise, if f did error, the
// transaction is rolled back. If the rollback fails, the original error
// returned by f is still returned. If the commit fails, the commit error is
// returned.
//
// NOTE: this is part of the PrivacyMapDB interface.
func (p *privacyMapDB) Update(ctx context.Context,
	f func(tx PrivacyMapTx) error) error {

	tx, err := p.db.beginTx(ctx, true)
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	err = f(tx)
	if err != nil {
		// Want to return the original error, not a rollback error if
		// any occur.
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// View opens a database read transaction and executes the function f with the
// transaction passed as a parameter. After f exits, the transaction is rolled
// back. If f errors, its error is returned, not a rollback error (if any
// occur).
//
// NOTE: this is part of the PrivacyMapDB interface.
func (p *privacyMapDB) View(ctx context.Context,
	f func(tx PrivacyMapTx) error) error {

	tx, err := p.db.beginTx(ctx, false)
	if err != nil {
		return err
	}

	// Make sure the transaction rolls back in the event of a panic.
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	err = f(tx)
	rollbackErr := tx.Rollback()
	if err != nil {
		return err
	}

	if rollbackErr != nil {
		return rollbackErr
	}
	return nil
}

func HideString(tx PrivacyMapTx, real string) (string, error) {
	pseudo, err := tx.RealToPseudo(real)
	if err != nil && !errors.Is(err, ErrNoSuchKeyFound) {
		return "", err
	}
	if err == nil {
		return pseudo, nil
	}

	pseudo, err = NewPseudoStr(len(real))
	if err != nil {
		return "", err
	}

	if err = tx.NewPair(real, pseudo); err != nil {
		return "", err
	}

	return pseudo, nil
}

func NewPseudoStr(n int) (string, error) {
	var max big.Int
	max.SetUint64(uint64(pseudoStrAlphabetLen))

	b := make([]rune, n)
	for i := range b {
		index, err := rand.Int(rand.Reader, &max)
		if err != nil {
			return "", err
		}

		b[i] = pseudoStrAlphabet[index.Uint64()]
	}

	return string(b), nil
}

func RevealString(tx PrivacyMapTx, pseudo string) (string, error) {
	if pseudo == "" {
		return pseudo, nil
	}

	return tx.PseudoToReal(pseudo)
}

func HideUint64(tx PrivacyMapTx, real uint64) (uint64, error) {
	str := Uint64ToStr(real)
	pseudo, err := tx.RealToPseudo(str)
	if err != nil && err != ErrNoSuchKeyFound {
		return 0, err
	}
	if err == nil {
		return StrToUint64(pseudo)
	}

	pseudoUint64, pseudoUint64Str := NewPseudoUint64()
	if err := tx.NewPair(str, pseudoUint64Str); err != nil {
		return 0, err
	}

	return pseudoUint64, nil
}

func RevealUint64(tx PrivacyMapTx, pseudo uint64) (uint64, error) {
	if pseudo == 0 {
		return 0, nil
	}

	real, err := tx.PseudoToReal(Uint64ToStr(pseudo))
	if err != nil {
		return 0, err
	}

	return StrToUint64(real)
}

func HideChanPoint(tx PrivacyMapTx, txid string, index uint32) (string,
	uint32, error) {

	cp := fmt.Sprintf("%s:%d", txid, index)
	pseudo, err := tx.RealToPseudo(cp)
	if err != nil && !errors.Is(err, ErrNoSuchKeyFound) {
		return "", 0, err
	}
	if err == nil {
		return DecodeChannelPoint(pseudo)
	}

	newCp, err := NewPseudoChanPoint()
	if err != nil {
		return "", 0, err
	}

	if err := tx.NewPair(cp, newCp); err != nil {
		return "", 0, err
	}

	return DecodeChannelPoint(newCp)
}

func NewPseudoChanPoint() (string, error) {
	pseudoTXID, err := NewPseudoStr(txidStringLen)
	if err != nil {
		return "", err
	}

	pseudoIndex := NewPseudoUint32()
	return fmt.Sprintf("%s:%d", pseudoTXID, pseudoIndex), nil
}

func RevealChanPoint(tx PrivacyMapTx, txid string, index uint32) (string,
	uint32, error) {

	fakePoint := fmt.Sprintf("%s:%d", txid, index)
	real, err := tx.PseudoToReal(fakePoint)
	if err != nil {
		return "", 0, err
	}

	return DecodeChannelPoint(real)
}

func NewPseudoUint32() uint32 {
	b := make([]byte, 4)
	_, _ = rand.Read(b)

	return binary.BigEndian.Uint32(b)
}

func HideChanPointStr(tx PrivacyMapTx, cp string) (string, error) {
	txid, index, err := DecodeChannelPoint(cp)
	if err != nil {
		return "", err
	}

	newTxid, newIndex, err := HideChanPoint(tx, txid, index)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", newTxid, newIndex), nil
}

func HideBytes(tx PrivacyMapTx, realBytes []byte) ([]byte, error) {
	real := hex.EncodeToString(realBytes)

	pseudo, err := HideString(tx, real)
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(pseudo)
}

func RevealBytes(tx PrivacyMapTx, pseudoBytes []byte) ([]byte, error) {
	if pseudoBytes == nil {
		return nil, nil
	}

	pseudo := hex.EncodeToString(pseudoBytes)
	pseudo, err := RevealString(tx, pseudo)
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(pseudo)
}

func NewPseudoUint64() (uint64, string) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)

	i := binary.BigEndian.Uint64(b)

	return i, hex.EncodeToString(b)
}

func Uint64ToStr(i uint64) string {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, i)
	return hex.EncodeToString(b)
}

func StrToUint64(s string) (uint64, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(b), nil
}

func DecodeChannelPoint(cp string) (string, uint32, error) {
	parts := strings.Split(cp, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("bad channel point encoding")
	}

	index, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, err
	}

	if len(parts[0]) != txidStringLen {
		return "", 0, fmt.Errorf("wrong txid length want %v, got %v",
			txidStringLen, len(parts[0]))
	}

	return parts[0], uint32(index), nil
}

// PrivacyMapReader is an interface that gives read access to a privacy map
// DB.
type PrivacyMapReader interface {
	// GetPseudo returns the associated pseudo value for a given real value.
	// If no such real value exists in the DB, then false is returned.
	GetPseudo(real string) (string, bool)
}

// PrivacyMapPairs is an in memory implementation of the PrivacyMapReader.
type PrivacyMapPairs struct {
	// pairs is a map from real to pseudo strings.
	pairs map[string]string

	mu sync.Mutex
}

// NewPrivacyMapPairs constructs a new PrivacyMapPairs struct. It may be
// initialised with either a nil map or a pre-defined real-to-pseudo strings
// map.
func NewPrivacyMapPairs(m map[string]string) *PrivacyMapPairs {
	if m != nil {
		return &PrivacyMapPairs{
			pairs: m,
		}
	}

	return &PrivacyMapPairs{
		pairs: make(map[string]string),
	}
}

// GetPseudo returns the associated pseudo value for a given real value. If no
// such real value exists in the DB, then false is returned.
//
// NOTE: this is part of the PrivacyMapReader interface.
func (p *PrivacyMapPairs) GetPseudo(real string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	pseudo, ok := p.pairs[real]

	return pseudo, ok
}

// Add adds the passed set of real-to-pseudo pairs to the PrivacyMapPairs
// structure. It will throw an error if the new pairs conflict with any of the
// existing pairs.
func (p *PrivacyMapPairs) Add(pairs map[string]string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Do a first pass to ensure that none of the new entries conflict with
	// the existing entries. We do this so that we don't mutate the set of
	// pairs before we know that the new set is valid.
	for realStr, pseudoStr := range pairs {
		ps, ok := p.pairs[realStr]
		if ok && ps != pseudoStr {
			return fmt.Errorf("cannot replace existing pseudo "+
				"entry for real value: %s", realStr)
		}
	}

	// In our second pass, we can add the new pairs to our set.
	for realStr, pseudoStr := range pairs {
		p.pairs[realStr] = pseudoStr
	}

	return nil
}
