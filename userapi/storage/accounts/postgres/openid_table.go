package postgres

import (
	"context"
	"database/sql"

	"github.com/matrix-org/dendrite/clientapi/userutil"
	"github.com/matrix-org/dendrite/internal/sqlutil"
	"github.com/matrix-org/dendrite/userapi/api"
	"github.com/matrix-org/gomatrixserverlib"
)

const openIDTokenSchema = `
-- Stores data about accounts.
CREATE TABLE IF NOT EXISTS account_openid (
	-- This is the token value, empty by default
	token TEXT NOT NULL PRIMARY KEY,
    -- The Matrix user ID localpart for this account
	localpart TEXT NOT NULL,
    -- When this token was first created, as a unix timestamp (ms resolution).
	token_created_ts BIGINT NOT NULL,
	-- When the token expires, as a unix timestamp (ms resolution).
	token_expires_ts BIGINT NOT NULL,
	-- (optional) Relying Party the token was created for
	token_rp TEXT,
);
-- Create sequence for autogenerated numeric usernames
-- CREATE SEQUENCE IF NOT EXISTS numeric_username_seq START 1;
`

const insertTokenSQL = "" +
	"INSERT INTO account_openid(token, localpart, token_created_ts, token_expires_ts, token_rp) VALUES ($1, $2, $3, $4, $5)"

const selectTokenSQL = "" +
	"SELECT token, localpart, token_created_ts, token_expires_ts, token_rp FROM account_openid WHERE token = $1"

type tokenStatements struct {
	insertTokenStmt *sql.Stmt
	selectTokenStmt *sql.Stmt
	serverName      gomatrixserverlib.ServerName
}

func (s *tokenStatements) prepare(db *sql.DB, server gomatrixserverlib.ServerName) (err error) {
	_, err = db.Exec(openIDTokenSchema)
	if err != nil {
		return
	}
	if s.insertTokenStmt, err = db.Prepare(insertTokenSQL); err != nil {
		return
	}
	if s.selectTokenStmt, err = db.Prepare(selectTokenSQL); err != nil {
		return
	}
	s.serverName = server
	return
}

func (s *tokenStatements) insertToken(
	ctx context.Context,
	txn *sql.Tx,
	token, localpart string,
	createdTimeMS, expiresTimeMS int64,
	tokenRP string,
) (err error) {
	stmt := sqlutil.TxStmt(txn, s.insertTokenStmt)

	if tokenRP == "" {
		_, err = stmt.ExecContext(ctx, token, localpart, createdTimeMS, expiresTimeMS, nil)
	} else {
		_, err = stmt.ExecContext(ctx, token, localpart, createdTimeMS, expiresTimeMS, tokenRP)
	}
	return
}

func (s *tokenStatements) selectToken(
	ctx context.Context,
	token string,
) (*api.OpenIDToken, error) {
	var openIDToken api.OpenIDToken
	var localpart string

	err := s.selectTokenStmt.QueryRowContext(ctx, token).Scan(
		&openIDToken.Token,
		localpart,
		&openIDToken.CreatedTS,
		&openIDToken.ExpiresTS,
		&openIDToken.RelyingParty,
	)
	if err != nil {
		return nil, err
	}

	openIDToken.UserID = userutil.MakeUserID(localpart, s.serverName)
	return &openIDToken, nil
}
