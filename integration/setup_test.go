//go:build integration

package integration

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"kronus.dev/commbadge_api/migrations"
)

var (
	pgPool    *pgxpool.Pool
	pgDSN     string
	redisURL  string
	cleanupFn func()
)

func setup(ctx context.Context) error {
	pgC, pgHost, pgPort, err := startPostgres(ctx)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	rdC, rdHost, rdPort, err := startRedis(ctx)
	if err != nil {
		pgC.Terminate(ctx)
		return fmt.Errorf("redis: %w", err)
	}

	cleanupFn = func() {
		pgC.Terminate(ctx)
		rdC.Terminate(ctx)
	}

	pgDSN = fmt.Sprintf("postgres://commbadge:commbadge@%s:%s/commbadge?sslmode=disable", pgHost, pgPort)
	version, applied, err := migrations.Apply(ctx, pgDSN)
	if err != nil {
		cleanupFn()
		return fmt.Errorf("migrations: %w", err)
	}
	log.Printf("migrations applied, version=%d applied=%d", version, applied)

	pool, err := pgxpool.New(ctx, pgDSN)
	if err != nil {
		cleanupFn()
		return fmt.Errorf("pgxpool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		cleanupFn()
		return fmt.Errorf("postgres ping: %w", err)
	}

	pgPool = pool
	redisURL = fmt.Sprintf("redis://%s:%s", rdHost, rdPort)

	log.Printf("Postgres ready at %s:%s, Redis ready at %s:%s", pgHost, pgPort, rdHost, rdPort)
	return nil
}

func teardown() {
	if pgPool != nil {
		pgPool.Close()
	}
	if cleanupFn != nil {
		cleanupFn()
	}
}

func startPostgres(ctx context.Context) (testcontainers.Container, string, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "commbadge",
			"POSTGRES_USER":     "commbadge",
			"POSTGRES_PASSWORD": "commbadge",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(30 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", "", err
	}
	host, err := c.Host(ctx)
	if err != nil {
		return nil, "", "", err
	}
	port, err := c.MappedPort(ctx, "5432")
	if err != nil {
		return nil, "", "", err
	}
	return c, host, port.Port(), nil
}

func startRedis(ctx context.Context) (testcontainers.Container, string, string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("* Ready to accept connections").WithStartupTimeout(30 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", "", err
	}
	host, err := c.Host(ctx)
	if err != nil {
		return nil, "", "", err
	}
	port, err := c.MappedPort(ctx, "6379")
	if err != nil {
		return nil, "", "", err
	}
	return c, host, port.Port(), nil
}
