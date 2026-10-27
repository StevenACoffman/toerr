package errpgx_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errhttp"
	"github.com/StevenACoffman/toerr/errors/errpgx"
)

// safeToRetryError is an error pgconn would report as having reached nothing on the
// server, such as a failure to acquire a connection.
type safeToRetryError struct{}

func (safeToRetryError) Error() string     { return "conn busy" }
func (safeToRetryError) SafeToRetry() bool { return true }

func TestFromSQLState(t *testing.T) {
	cases := map[string]struct {
		state string
		want  errcode.StatusCode
	}{
		"unique violation":   {pgerrcode.UniqueViolation, errcode.StatusAlreadyExists},
		"not null violation": {pgerrcode.NotNullViolation, errcode.StatusInvalidArgument},
		"check violation":    {pgerrcode.CheckViolation, errcode.StatusInvalidArgument},
		"foreign key violation": {
			pgerrcode.ForeignKeyViolation,
			errcode.StatusFailedPrecondition,
		},
		"exclusion violation": {
			pgerrcode.ExclusionViolation,
			errcode.StatusFailedPrecondition,
		},
		"data exception": {
			pgerrcode.InvalidTextRepresentation,
			errcode.StatusInvalidArgument,
		},
		"serialization failure":    {pgerrcode.SerializationFailure, errcode.StatusAborted},
		"deadlock":                 {pgerrcode.DeadlockDetected, errcode.StatusAborted},
		"commit unknown":           {pgerrcode.StatementCompletionUnknown, errcode.StatusUnknown},
		"lock not available":       {pgerrcode.LockNotAvailable, errcode.StatusAborted},
		"statement timeout":        {pgerrcode.QueryCanceled, errcode.StatusDeadlineExceeded},
		"admin shutdown":           {pgerrcode.AdminShutdown, errcode.StatusUnavailable},
		"cannot connect now":       {pgerrcode.CannotConnectNow, errcode.StatusUnavailable},
		"idle session timeout":     {pgerrcode.IdleSessionTimeout, errcode.StatusUnavailable},
		"read-only after failover": {pgerrcode.ReadOnlySQLTransaction, errcode.StatusUnavailable},
		"connection exception":     {pgerrcode.ConnectionFailure, errcode.StatusUnavailable},
		"too many connections":     {pgerrcode.TooManyConnections, errcode.StatusResourceExhausted},
		"disk full":                {pgerrcode.DiskFull, errcode.StatusResourceExhausted},
		"data corrupted":           {pgerrcode.DataCorrupted, errcode.StatusDataLoss},
		"index corrupted":          {pgerrcode.IndexCorrupted, errcode.StatusDataLoss},
		// The database's own credentials and grants are the program's configuration,
		// not the caller's identity.
		"bad password":           {pgerrcode.InvalidPassword, errcode.StatusInternal},
		"insufficient privilege": {pgerrcode.InsufficientPrivilege, errcode.StatusInternal},
		"undefined table":        {pgerrcode.UndefinedTable, errcode.StatusInternal},
		"in failed transaction":  {pgerrcode.InFailedSQLTransaction, errcode.StatusInternal},
		"internal error":         {pgerrcode.InternalError, errcode.StatusInternal},
		// A code a later PostgreSQL adds to a known class is classified by its class.
		"new code in a known class": {"40ZZZ", errcode.StatusAborted},
		"unknown class":             {pgerrcode.InvalidCatalogName, errcode.StatusUnknown},
		"too short":                 {"4", errcode.StatusUnknown},
		"empty":                     {"", errcode.StatusUnknown},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errpgx.FromSQLState(tc.state); got != tc.want {
				t.Errorf("FromSQLState(%q) = %v, want %v", tc.state, got, tc.want)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	cases := map[string]struct {
		err  error
		want errcode.StatusCode
	}{
		"nil":         {nil, errcode.StatusUnknown},
		"no rows":     {pgx.ErrNoRows, errcode.StatusNotFound},
		"sql no rows": {fmt.Errorf("scan: %w", sql.ErrNoRows), errcode.StatusNotFound},
		"server error": {
			fmt.Errorf("insert: %w", &pgconn.PgError{Code: pgerrcode.UniqueViolation}),
			errcode.StatusAlreadyExists,
		},
		"canceled": {fmt.Errorf("query: %w", context.Canceled), errcode.StatusCanceled},
		"deadline": {
			fmt.Errorf("query: %w", context.DeadlineExceeded),
			errcode.StatusDeadlineExceeded,
		},
		"commit rolled back": {pgx.ErrTxCommitRollback, errcode.StatusAborted},
		"too many rows":      {pgx.ErrTooManyRows, errcode.StatusInternal},
		"closed tx":          {pgx.ErrTxClosed, errcode.StatusInternal},
		"safe to retry": {
			fmt.Errorf("acquire: %w", safeToRetryError{}),
			errcode.StatusUnavailable,
		},
		"unclassified": {errors.New("boom"), errcode.StatusUnknown},
		// pgx reports a query it cancelled for a context as 57014 too; the context wins.
		"cancelled query": {
			errors.Join(&pgconn.PgError{Code: pgerrcode.QueryCanceled}, context.Canceled),
			errcode.StatusCanceled,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errpgx.Status(tc.err); got != tc.want {
				t.Errorf("Status(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// Connection failures come from real sockets: pgx's timeout error cannot be built
// outside pgconn.
func TestStatusOfConnectionFailures(t *testing.T) {
	cases := map[string]struct {
		run  func(t *testing.T) error
		want errcode.StatusCode
	}{
		// Nothing ran, so the operation can be retried elsewhere.
		"refused":           {connectToClosedPort, errcode.StatusUnavailable},
		"handshake timeout": {connectToSilentServer, errcode.StatusUnavailable},
		// The statement was sent and may have run.
		"statement timeout": {queryUnansweredStatement, errcode.StatusDeadlineExceeded},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.run(t)
			if got := errpgx.Status(err); got != tc.want {
				t.Errorf("Status(%v) = %v, want %v", err, got, tc.want)
			}
		})
	}
}

// A translated error is sealed, which hides the context error from
// errcode.ForTransport, so the code Status attached is what reaches the client.
func TestSealedTranslationAnswersAtTheBoundary(t *testing.T) {
	cases := map[string]struct {
		err  error
		want int
	}{
		"canceled": {
			fmt.Errorf("query: %w", context.Canceled),
			errhttp.StatusClientClosedRequest,
		},
		"duplicate":    {&pgconn.PgError{Code: pgerrcode.UniqueViolation}, http.StatusConflict},
		"unclassified": {errors.New("boom"), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			translated := errcode.Link(errpgx.Status(tc.err), "", toerr.Seal(tc.err))
			if status, _ := errhttp.Error(translated); status != tc.want {
				t.Errorf("errhttp status = %d, want %d", status, tc.want)
			}
			var pgErr *pgconn.PgError
			if errors.As(translated, &pgErr) {
				t.Error("the sealed *pgconn.PgError is still reachable by errors.As")
			}
		})
	}
}

func connectToClosedPort(t *testing.T) error {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	_, err = pgconn.Connect(t.Context(), "postgres://u@"+addr+"/db?sslmode=disable")
	return err
}

func connectToSilentServer(t *testing.T) error {
	t.Helper()
	addr := serve(t, func(net.Conn) {})
	cfg := parseConfig(t, addr)
	cfg.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		_ = c.SetDeadline(time.Now().Add(50 * time.Millisecond))
		return c, nil
	}
	_, err := pgconn.ConnectConfig(t.Context(), cfg)
	return err
}

func queryUnansweredStatement(t *testing.T) error {
	t.Helper()
	addr := serve(t, func(c net.Conn) {
		b := pgproto3.NewBackend(c, c)
		if _, err := b.ReceiveStartupMessage(); err != nil {
			return
		}
		b.Send(&pgproto3.AuthenticationOk{})
		b.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
		b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		_ = b.Flush()
	})
	conn, err := pgconn.ConnectConfig(t.Context(), parseConfig(t, addr))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Conn().Close() })
	_ = conn.Conn().SetDeadline(time.Now().Add(50 * time.Millisecond))
	_, err = conn.Exec(t.Context(), "select 1").ReadAll()
	return err
}

// serve accepts one connection on a loopback port, runs handle on it, and then holds
// it open, unanswered, until the client closes it.
func serve(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		handle(c)
		_, _ = io.Copy(io.Discard, c)
	}()
	return ln.Addr().String()
}

func parseConfig(t *testing.T, addr string) *pgconn.Config {
	t.Helper()
	cfg, err := pgconn.ParseConfig("postgres://u@" + addr + "/db?sslmode=disable")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return cfg
}
