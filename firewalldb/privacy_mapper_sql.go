package firewalldb

import (
	"context"

	"github.com/lightninglabs/lightning-terminal/db/sqlc"
)

type SQLPrivacyPairQueries interface {
	SQLSessionQueries

	InsertPrivacyPair(ctx context.Context, arg sqlc.InsertPrivacyPairParams) error
	GetAllPrivacyPairs(ctx context.Context, groupID int64) ([]sqlc.GetAllPrivacyPairsRow, error)
	GetPseudoForReal(ctx context.Context, arg sqlc.GetPseudoForRealParams) (string, error)
	GetRealForPseudo(ctx context.Context, arg sqlc.GetRealForPseudoParams) (string, error)
}
