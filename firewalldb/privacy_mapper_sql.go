package firewalldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lightninglabs/lightning-terminal/db"
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

type SQLPrivacyPairDB struct {
	*db.BaseDB
}

var _ PrivMapDBCreator = (*SQLPrivacyPairDB)(nil)

func NewSQLPrivacyPairDB(db *db.BaseDB) *SQLPrivacyPairDB {
	return &SQLPrivacyPairDB{
		BaseDB: db,
	}
}

func (s *SQLPrivacyPairDB) PrivacyDB(groupID session.ID) PrivacyMapDB {
	return &dbExecutor[PrivacyMapTx]{
		db: &privacyMapSQLDB{
			BaseDB:  s.BaseDB,
			groupID: groupID,
		},
	}
}

type privacyMapSQLDB struct {
	*db.BaseDB
	groupID session.ID
}

var _ txCreator[PrivacyMapTx] = (*privacyMapKVDBDB)(nil)

// beginTx starts db transaction. The transaction will be a read or read-write
// transaction depending on the value of the `writable` parameter.
func (s *privacyMapSQLDB) beginTx(ctx context.Context, writable bool) (
	PrivacyMapTx, error) {

	var txOpts db.QueriesTxOptions
	if !writable {
		txOpts = db.NewQueryReadTx()
	}

	sqlTX, err := s.BeginTx(ctx, &txOpts)
	if err != nil {
		return nil, err
	}

	return &privacyMapSQLTx{
		db: s,
		Tx: sqlTX,
	}, nil
}

// privacyMapSQLTx is an implementation of PrivacyMapTx.
type privacyMapSQLTx struct {
	db *privacyMapSQLDB
	*sql.Tx
}

func (p *privacyMapSQLTx) IsNil() bool {
	return p.Tx == nil
}

func (p *privacyMapSQLTx) NewPair(real, pseudo string) error {
	ctx := context.TODO()
	queries := SQLPrivacyPairQueries(p.db.WithTx(p.Tx))

	groupID, err := queries.GetSessionIDByLegacyID(ctx, p.db.groupID[:])
	if err != nil {
		return err
	}

	_, err = queries.GetPseudoForReal(ctx, sqlc.GetPseudoForRealParams{
		GroupID: groupID,
		Real:    real,
	})
	if err == nil {
		return ErrDuplicateRealValue
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = queries.GetRealForPseudo(ctx, sqlc.GetRealForPseudoParams{
		GroupID: groupID,
		Pseudo:  pseudo,
	})
	if err == nil {
		return ErrDuplicatePseudoValue
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	return queries.InsertPrivacyPair(ctx, sqlc.InsertPrivacyPairParams{
		GroupID: groupID,
		Real:    real,
		Pseudo:  pseudo,
	})
}

func (p *privacyMapSQLTx) PseudoToReal(pseudo string) (string, error) {
	ctx := context.TODO()
	queries := SQLPrivacyPairQueries(p.db.WithTx(p.Tx))

	groupID, err := queries.GetSessionIDByLegacyID(ctx, p.db.groupID[:])
	if err != nil {
		return "", err
	}

	real, err := queries.GetRealForPseudo(ctx, sqlc.GetRealForPseudoParams{
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

func (p *privacyMapSQLTx) RealToPseudo(real string) (string, error) {
	ctx := context.TODO()
	queries := SQLPrivacyPairQueries(p.db.WithTx(p.Tx))

	groupID, err := queries.GetSessionIDByLegacyID(ctx, p.db.groupID[:])
	if errors.Is(err, sql.ErrNoRows) {
		return "", session.ErrUnknownGroup
	} else if err != nil {
		return "", err
	}

	pseudo, err := queries.GetPseudoForReal(ctx, sqlc.GetPseudoForRealParams{
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

func (p *privacyMapSQLTx) FetchAllPairs() (*PrivacyMapPairs, error) {
	ctx := context.TODO()
	queries := SQLPrivacyPairQueries(p.db.WithTx(p.Tx))

	groupID, err := queries.GetSessionIDByLegacyID(ctx, p.db.groupID[:])
	if err != nil {
		return nil, err
	}

	pairs, err := queries.GetAllPrivacyPairs(ctx, groupID)
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
