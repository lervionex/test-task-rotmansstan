package testutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"test-task-rotmansstan/internal/platform/database"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func NewTestPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	containerName := fmt.Sprintf("withdrawals-test-%d", time.Now().UnixNano())

	runCmd := exec.CommandContext(
		ctx,
		"docker",
		"run",
		"-d",
		"--rm",
		"--name", containerName,
		"-e", "POSTGRES_DB=withdrawals_test",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-P",
		"postgres:16-alpine",
	)

	output, err := runCmd.CombinedOutput()
	require.NoError(t, err, string(output))

	t.Cleanup(func() {
		stopCmd := exec.Command("docker", "rm", "-f", containerName)
		_, _ = stopCmd.CombinedOutput()
	})

	portCmd := exec.CommandContext(ctx, "docker", "port", containerName, "5432/tcp")
	portOutput, err := portCmd.CombinedOutput()
	require.NoError(t, err, string(portOutput))

	lines := strings.Split(strings.TrimSpace(string(portOutput)), "\n")
	require.NotEmpty(t, lines)

	hostPort := strings.TrimSpace(lines[0])
	index := strings.LastIndex(hostPort, ":")
	require.NotEqual(t, -1, index, hostPort)

	connString := fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%s/withdrawals_test?sslmode=disable", hostPort[index+1:])

	var pool *pgxpool.Pool
	require.Eventually(t, func() bool {
		var pingErr error
		pool, pingErr = pgxpool.New(ctx, connString)
		if pingErr != nil {
			return false
		}

		pingCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		if pingErr = pool.Ping(pingCtx); pingErr != nil {
			pool.Close()
			return false
		}

		return true
	}, 60*time.Second, 500*time.Millisecond)

	t.Cleanup(pool.Close)

	require.NoError(t, database.Migrate(ctx, pool))

	return pool
}
