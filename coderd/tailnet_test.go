package coderd_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"cdr.dev/slog"
	"cdr.dev/slog/sloggers/slogtest"
	"github.com/coder/coder/agent"
	"github.com/coder/coder/agent/agenttest"
	"github.com/coder/coder/coderd"
	"github.com/coder/coder/coderd/wsconncache"
	"github.com/coder/coder/codersdk"
	"github.com/coder/coder/codersdk/agentsdk"
	"github.com/coder/coder/tailnet"
	"github.com/coder/coder/tailnet/tailnettest"
	"github.com/coder/coder/testutil"
)

func TestServerTailnet_AgentConn_OK(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitMedium)
	defer cancel()

	// Connect through the ServerTailnet
	agentID, _, serverTailnet := setupAgent(t, nil)

	conn, release, err := serverTailnet.AgentConn(ctx, agentID)
	require.NoError(t, err)
	defer release()

	assert.True(t, conn.AwaitReachable(ctx))
}

func TestServerTailnet_AgentConn_Legacy(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitMedium)
	defer cancel()

	// Force a connection through wsconncache using the legacy hardcoded ip.
	agentID, _, serverTailnet := setupAgent(t, []netip.Prefix{
		netip.PrefixFrom(codersdk.WorkspaceAgentIP, 128),
	})

	conn, release, err := serverTailnet.AgentConn(ctx, agentID)
	require.NoError(t, err)
	defer release()

	assert.True(t, conn.AwaitReachable(ctx))
}

func TestServerTailnet_ReverseProxy(t *testing.T) {
	t.Parallel()

	t.Run("OK", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		agentID, _, serverTailnet := setupAgent(t, nil)

		u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", codersdk.WorkspaceAgentHTTPAPIServerPort))
		require.NoError(t, err)

		rp, release, err := serverTailnet.ReverseProxy(u, u, agentID)
		require.NoError(t, err)
		defer release()

		rw := httptest.NewRecorder()
		req := httptest.NewRequest(
			http.MethodGet,
			u.String(),
			nil,
		).WithContext(ctx)

		rp.ServeHTTP(rw, req)
		res := rw.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("HTTPSProxy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		agentID, _, serverTailnet := setupAgent(t, nil)

		const expectedResponseCode = 209
		// Test that we can proxy HTTPS traffic.
		s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(expectedResponseCode)
		}))
		t.Cleanup(s.Close)

		uri, err := url.Parse(s.URL)
		require.NoError(t, err)

		rp, release, err := serverTailnet.ReverseProxy(uri, uri, agentID)
		require.NoError(t, err)
		defer release()

		rw := httptest.NewRecorder()
		req := httptest.NewRequest(
			http.MethodGet,
			uri.String(),
			nil,
		).WithContext(ctx)

		rp.ServeHTTP(rw, req)
		res := rw.Result()
		defer res.Body.Close()

		assert.Equal(t, expectedResponseCode, res.StatusCode)
	})

	t.Run("Legacy", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), testutil.WaitLong)
		defer cancel()

		// Force a connection through wsconncache using the legacy hardcoded ip.
		agentID, _, serverTailnet := setupAgent(t, []netip.Prefix{
			netip.PrefixFrom(codersdk.WorkspaceAgentIP, 128),
		})

		u, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", codersdk.WorkspaceAgentHTTPAPIServerPort))
		require.NoError(t, err)

		rp, release, err := serverTailnet.ReverseProxy(u, u, agentID)
		require.NoError(t, err)
		defer release()

		rw := httptest.NewRecorder()
		req := httptest.NewRequest(
			http.MethodGet,
			u.String(),
			nil,
		).WithContext(ctx)

		rp.ServeHTTP(rw, req)
		res := rw.Result()
		defer res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
	})
}

func setupAgent(t *testing.T, agentAddresses []netip.Prefix) (uuid.UUID, agent.Agent, *coderd.ServerTailnet) {
	logger := slogtest.Make(t, nil).Leveled(slog.LevelDebug)
	derpMap, derpServer := tailnettest.RunDERPAndSTUN(t)
	manifest := agentsdk.Manifest{
		AgentID: uuid.New(),
		DERPMap: derpMap,
	}

	coord := tailnet.NewCoordinator(logger)
	t.Cleanup(func() {
		_ = coord.Close()
	})

	c := agenttest.NewClient(t, logger, manifest.AgentID, manifest, make(chan *agentsdk.Stats, 50), coord)

	options := agent.Options{
		Client:     c,
		Filesystem: afero.NewMemMapFs(),
		Logger:     logger.Named("agent"),
		Addresses:  agentAddresses,
	}

	ag := agent.New(options)
	t.Cleanup(func() {
		_ = ag.Close()
	})

	// Wait for the agent to connect.
	require.Eventually(t, func() bool {
		return coord.Node(manifest.AgentID) != nil
	}, testutil.WaitShort, testutil.IntervalFast)

	cache := wsconncache.New(func(id uuid.UUID) (*codersdk.WorkspaceAgentConn, error) {
		conn, err := tailnet.NewConn(&tailnet.Options{
			Addresses: []netip.Prefix{netip.PrefixFrom(tailnet.IP(), 128)},
			DERPMap:   manifest.DERPMap,
			Logger:    logger.Named("client"),
		})
		require.NoError(t, err)
		clientConn, serverConn := net.Pipe()
		serveClientDone := make(chan struct{})
		t.Cleanup(func() {
			_ = clientConn.Close()
			_ = serverConn.Close()
			_ = conn.Close()
			<-serveClientDone
		})
		go func() {
			defer close(serveClientDone)
			coord.ServeClient(serverConn, uuid.New(), manifest.AgentID)
		}()
		sendNode, _ := tailnet.ServeCoordinator(clientConn, func(node []*tailnet.Node) error {
			return conn.UpdateNodes(node, false)
		})
		conn.SetNodeCallback(sendNode)
		return codersdk.NewWorkspaceAgentConn(conn, codersdk.WorkspaceAgentConnOptions{
			AgentID:   manifest.AgentID,
			AgentIP:   codersdk.WorkspaceAgentIP,
			CloseFunc: func() error { return codersdk.ErrSkipClose },
		}), nil
	}, 0)

	serverTailnet, err := coderd.NewServerTailnet(
		context.Background(),
		logger,
		derpServer,
		manifest.DERPMap,
		func(context.Context) (tailnet.MultiAgentConn, error) { return coord.ServeMultiAgent(uuid.New()), nil },
		cache,
		trace.NewNoopTracerProvider(),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = serverTailnet.Close()
	})

	return manifest.AgentID, ag, serverTailnet
}
