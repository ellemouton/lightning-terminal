package firewalldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lightninglabs/lightning-terminal/db/sqlc"
	"github.com/lightninglabs/lightning-terminal/session"
)

type SQLPrivacyPairQueries interface {
	SQLSessionQueries

	InsertPrivacyPair(ctx context.Context, arg sqlc.InsertPrivacyPairParams) error
	GetAllPrivacyPairs(ctx context.Context, groupID int64) ([]sqlc.GetAllPrivacyPairsRow, error)
	GetPseudoForReal(ctx context.Context, arg sqlc.GetPseudoForRealParams) (string, error)
	GetRealForPseudo(ctx context.Context, arg sqlc.GetRealForPseudoParams) (string, error)
}

func (s *SQLDB) PrivacyDB(groupID session.ID) PrivacyMapDB {
	return &sqlExecutor[PrivacyMapTx]{
		db: s.db,
		wrapTx: func(queries SQLQueries) PrivacyMapTx {
			return &privacyMapSQLTx{
				queries: queries,
				db: &privacyMapSQLDB{
					SQLDB:   s,
					groupID: groupID,
				},
			}
		},
	}
}

type privacyMapSQLDB struct {
	*SQLDB
	groupID session.ID
}

// privacyMapSQLTx is an implementation of PrivacyMapTx.
type privacyMapSQLTx struct {
	db      *privacyMapSQLDB
	queries SQLQueries
}

func (p *privacyMapSQLTx) NewPair(ctx context.Context, real,
	pseudo string) error {

	groupID, err := p.queries.GetSessionIDByAlias(ctx, p.db.groupID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return session.ErrUnknownGroup
	} else if err != nil {
		return err
	}

	_, err = p.queries.GetPseudoForReal(ctx, sqlc.GetPseudoForRealParams{
		GroupID: groupID,
		Real:    real,
	})
	if err == nil {
		return ErrDuplicateRealValue
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = p.queries.GetRealForPseudo(ctx, sqlc.GetRealForPseudoParams{
		GroupID: groupID,
		Pseudo:  pseudo,
	})
	if err == nil {
		return ErrDuplicatePseudoValue
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	return p.queries.InsertPrivacyPair(ctx, sqlc.InsertPrivacyPairParams{
		GroupID: groupID,
		Real:    real,
		Pseudo:  pseudo,
	})
}

func (p *privacyMapSQLTx) PseudoToReal(ctx context.Context,
	pseudo string) (string, error) {

	groupID, err := p.queries.GetSessionIDByAlias(ctx, p.db.groupID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return "", session.ErrUnknownGroup
	} else if err != nil {
		return "", err
	}

	real, err := p.queries.GetRealForPseudo(ctx, sqlc.GetRealForPseudoParams{
		GroupID: groupID,
		Pseudo:  pseudo,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoSuchKeyFound
	} else if err != nil {
		return "", err
	}

	return real, nil
}

func (p *privacyMapSQLTx) RealToPseudo(ctx context.Context,
	real string) (string, error) {

	groupID, err := p.queries.GetSessionIDByAlias(ctx, p.db.groupID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return "", session.ErrUnknownGroup
	} else if err != nil {
		return "", err
	}

	pseudo, err := p.queries.GetPseudoForReal(ctx, sqlc.GetPseudoForRealParams{
		GroupID: groupID,
		Real:    real,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNoSuchKeyFound
	} else if err != nil {
		return "", err
	}

	return pseudo, nil
}

func (p *privacyMapSQLTx) FetchAllPairs(ctx context.Context) (*PrivacyMapPairs,
	error) {

	groupID, err := p.queries.GetSessionIDByAlias(ctx, p.db.groupID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return nil, session.ErrUnknownGroup
	} else if err != nil {
		return nil, err
	}

	pairs, err := p.queries.GetAllPrivacyPairs(ctx, groupID)
	if err != nil {
		return nil, err
	}

	privacyPairs := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		privacyPairs[pair.Real] = pair.Pseudo
	}

	return NewPrivacyMapPairs(privacyPairs), nil
}

var _ PrivacyMapTx = (*privacyMapSQLTx)(nil)
