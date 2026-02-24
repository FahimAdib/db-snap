package db

import (
	"context"
	"fmt"
	"net/url"

	"db-snap/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnString(p model.DBProfile, password string) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(p.User, password),
		Host:   fmt.Sprintf("%s:%d", p.Host, p.Port),
		Path:   p.Database,
	}
	q := u.Query()
	sslMode := p.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func Ping(ctx context.Context, p model.DBProfile, password string) error {
	conn, err := pgx.Connect(ctx, ConnString(p, password))
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	return conn.Ping(ctx)
}

func OpenPool(ctx context.Context, p model.DBProfile, password string) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(ConnString(p, password))
	if err != nil {
		return nil, err
	}
	pcfg.MaxConns = 5
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
