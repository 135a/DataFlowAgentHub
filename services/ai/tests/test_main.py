"""Tests for hub_ai/__main__.py"""

import os
import sys
import json
import logging
import importlib
import pytest
from unittest.mock import patch, MagicMock, PropertyMock

# Mock grpc at module level so we can import hub_ai.__main__
_grpc_mock = MagicMock()
_futures_mock = MagicMock()
sys.modules["grpc"] = _grpc_mock
sys.modules["concurrent"] = _futures_mock
sys.modules["concurrent.futures"] = _futures_mock


class TestReadOnlyOk:
    """Test _read_only_ok — a pure string validation function."""

    def _import(self):
        """Import the main module (grpc is already mocked at module scope)."""
        for mod in list(sys.modules.keys()):
            if mod.startswith("hub_ai.__main__"):
                del sys.modules[mod]
        import hub_ai.__main__ as m
        return m

    def test_valid_sql_returns_ok(self):
        m = self._import()
        ok, msg = m._read_only_ok("SELECT * FROM test")
        assert ok is True
        assert msg == ""

    def test_empty_string_returns_false(self):
        m = self._import()
        ok, msg = m._read_only_ok("")
        assert ok is False
        assert "empty" in msg

    def test_whitespace_only_returns_false(self):
        m = self._import()
        ok, msg = m._read_only_ok("   \n  ")
        assert ok is False
        assert "empty" in msg

    def test_with_semicolon(self):
        m = self._import()
        ok, msg = m._read_only_ok("SELECT 1;")
        assert ok is True
        assert msg == ""

    def test_write_keywords_not_blocked(self):
        """Write keywords are no longer blocked (Go side handles this)."""
        m = self._import()
        for sql in ["INSERT INTO t VALUES (1)", "DELETE FROM t", "DROP TABLE t"]:
            ok, msg = m._read_only_ok(sql)
            assert ok is True, f"Should allow: {sql}"

    def test_case_insensitive_upper_strip(self):
        m = self._import()
        ok, msg = m._read_only_ok("  select 1  ")
        assert ok is True


class TestSetupLogging:
    """Test _setup_logging configuration."""

    def _import(self):
        for mod in list(sys.modules.keys()):
            if mod.startswith("hub_ai.__main__"):
                del sys.modules[mod]
        import hub_ai.__main__ as m
        return m

    def test_default_log_level(self):
        """Default is INFO when LOG_LEVEL is not set."""
        m = self._import()
        with patch.dict(os.environ, {}, clear=True):
            with patch("logging.basicConfig") as mock_cfg:
                m._setup_logging()
                _, kwargs = mock_cfg.call_args
                assert kwargs["level"] == logging.INFO

    def test_custom_log_level(self):
        """Should respect LOG_LEVEL env var."""
        m = self._import()
        with patch.dict(os.environ, {"LOG_LEVEL": "DEBUG"}, clear=True):
            with patch("logging.basicConfig") as mock_cfg:
                m._setup_logging()
                _, kwargs = mock_cfg.call_args
                assert kwargs["level"] == logging.DEBUG

    def test_json_format(self):
        """Log format should contain %(message)s."""
        m = self._import()
        with patch("logging.basicConfig") as mock_cfg:
            m._setup_logging()
            _, kwargs = mock_cfg.call_args
            assert kwargs["format"] is not None
            assert "%(message)s" in kwargs["format"]


class TestMain:
    """Test main() function — gRPC server and NATS consumer threads."""

    @pytest.fixture(autouse=True)
    def _setup_mocks(self):
        """Configure global mocks before each test."""
        # Reset grpc mock state
        _grpc_mock.reset_mock()
        _grpc_mock.server.reset_mock()
        _grpc_mock.ssl_server_credentials = MagicMock()

        # Mock nl2sql_pb2 and pb2_grpc
        self.mock_pb2 = MagicMock()
        self.mock_pb2_grpc = MagicMock()

        # Mock threading
        self.mock_threading = MagicMock()
        self.mock_thread = MagicMock()
        self.mock_threading.Thread = self.mock_thread

        # Mock asyncio
        self.mock_asyncio = MagicMock()

        # Mock os.path functions
        self.mock_abspath = MagicMock(return_value="/fake/path/gen")
        self.mock_exists = MagicMock(return_value=True)

    def _reimport(self):
        """Re-import main module with fresh mocks."""
        for mod in list(sys.modules.keys()):
            if mod.startswith("hub_ai.__main__"):
                del sys.modules[mod]
            if mod.startswith("nl2sql"):
                del sys.modules[mod]

        with patch.dict("sys.modules", {
            "nl2sql.v1.nl2sql_pb2": self.mock_pb2,
            "nl2sql.v1.nl2sql_pb2_grpc": self.mock_pb2_grpc,
            "threading": self.mock_threading,
            "asyncio": self.mock_asyncio,
        }), \
             patch("os.path.abspath", self.mock_abspath), \
             patch("os.path.exists", self.mock_exists):
            import hub_ai.__main__ as m
            return m

    def test_creates_grpc_server_with_thread_pool(self):
        """Should create a gRPC server with ThreadPoolExecutor."""
        m = self._reimport()
        with patch.dict(os.environ, {}, clear=True):
            m.main()
            _grpc_mock.server.assert_called_once()

    def test_adds_insecure_port_by_default(self):
        """Without mTLS certs, should use insecure port."""
        m = self._reimport()
        with patch.dict(os.environ, {}, clear=True):
            m.main()
            _grpc_mock.server.return_value.add_insecure_port.assert_called_once()

    def test_server_starts_and_waits(self):
        """Server.start() and wait_for_termination() should be called."""
        m = self._reimport()
        with patch.dict(os.environ, {}, clear=True):
            m.main()
            mock_server = _grpc_mock.server.return_value
            mock_server.start.assert_called_once()
            mock_server.wait_for_termination.assert_called_once()

    def test_starts_two_consumer_threads(self):
        """Two daemon NATS consumer threads should be started."""
        m = self._reimport()
        with patch.dict(os.environ, {}, clear=True):
            m.main()
            assert self.mock_thread.call_count >= 2
            for call_args in self.mock_thread.call_args_list:
                _, kwargs = call_args
                assert kwargs.get("daemon") is True

    def test_adds_servicer_to_server(self):
        """NL2SQLServiceServicer should be added to the server."""
        m = self._reimport()
        with patch.dict(os.environ, {}, clear=True):
            m.main()
            self.mock_pb2_grpc.add_NL2SQLServiceServicer_to_server.assert_called_once()

    def test_uses_configured_addr(self):
        """Should use WORKER_GRPC_ADDR env var for binding."""
        m = self._reimport()
        with patch.dict(os.environ, {"WORKER_GRPC_ADDR": "0.0.0.0:9999"}, clear=True):
            m.main()
            mock_server = _grpc_mock.server.return_value
            mock_server.add_insecure_port.assert_called_once_with("0.0.0.0:9999")

    def test_enables_mtls_with_certs(self):
        """Should use secure port when mTLS certs are configured."""
        m = self._reimport()
        with patch.dict(os.environ, {
            "HUB_GRPC_CA_CERT": "/certs/ca.pem",
            "HUB_GRPC_SERVER_CERT": "/certs/server.pem",
            "HUB_GRPC_SERVER_KEY": "/certs/server.key",
        }, clear=True), \
             patch("builtins.open", MagicMock()) as mock_open:
            mock_file = MagicMock()
            mock_file.__enter__.return_value.read.return_value = b"cert_bytes"
            mock_open.return_value = mock_file

            m.main()
            mock_server = _grpc_mock.server.return_value
            # add_secure_port should be used, not add_insecure_port
            mock_server.add_secure_port.assert_called_once()
            assert mock_server.add_insecure_port.call_count == 0
