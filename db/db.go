package db

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type Client struct {
	*sqlx.DB
}

func Open(ctx context.Context, dsn string, opts ...Option) (*Client, error) {
	db := &Client{}
	for _, opt := range opts {
		opt(db)
	}

	pgDb, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, errors.WithMessage(err, "open database with pgx driver")
	}

	pgDb.MapperFunc(ToSnakeCase)
	err = pgDb.PingContext(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "ping database")
	}

	db.DB = pgDb
	return db, nil
}

func (db *Client) RunInTransaction(ctx context.Context, txFunc TxFunc, opts ...TxOption) (err error) {
	options := &txOptions{}
	for _, opt := range opts {
		opt(options)
	}
	tx, err := db.BeginTxx(ctx, options.nativeOpts)
	if err != nil {
		return errors.WithMessage(err, "begin transaction")
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = errors.Wrap(err, rbErr.Error())
			}
		} else {
			err = errors.Wrap(tx.Commit(), "commit tx")
		}
	}()

	return txFunc(ctx, &Tx{tx})
}

func (db *Client) Select(ctx context.Context, ptr interface{}, query string, args ...interface{}) error {
	return db.SelectContext(ctx, ptr, query, args...)
}

func (db *Client) SelectOne(ctx context.Context, ptr interface{}, query string, args ...interface{}) error {
	return db.GetContext(ctx, ptr, query, args...)
}

func (db *Client) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.ExecContext(ctx, query, args...)
}

func (db *Client) ExecNamed(ctx context.Context, query string, args interface{}) (sql.Result, error) {
	return db.NamedExecContext(ctx, query, args)
}
