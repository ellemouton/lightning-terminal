-- name: InsertPrivacyPair :exec
INSERT INTO privacy_pairs (group_id, real, pseudo)
VALUES ($1, $2, $3);

-- name: GetRealForPseudo :one
SELECT real
FROM privacy_pairs
WHERE group_id = $1 AND pseudo = $2;

-- name: GetPseudoForReal :one
SELECT pseudo
FROM privacy_pairs
WHERE group_id = $1 AND real = $2;

-- name: GetAllPrivacyPairs :many
SELECT real, pseudo
FROM privacy_pairs
WHERE group_id = $1;
