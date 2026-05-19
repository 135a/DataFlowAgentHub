"""Tests for hub_ai/__main__.py (async gRPC architecture)."""

import os
import sys
import logging
import pytest
from unittest.mock import patch, MagicMock, AsyncMock

# Mock grpc at module level so we can import without real gRPC
_grpc_mock = MagicMock()
sys.modules["grpc"] = _grpc_mock
_grpc_aio_mock = MagicMock()
sys.modules["grpc.aio"] = _grpc_aio_mock
_grpc_mock.aio = _grpc_aio_mock  # so "from grpc import aio" resolves to our mock


class TestSetupLogging:
    """Test _setup_logging configuration."""

    def _import(self):
        for mod in list(sys.modules.keys()):
            if mod.startswith("hub_ai.__main__"):
                del sys.modules[mod]
        import hub_ai.__main__ as m
        return m

    def test_default_log_level(self):
        m = self._import()
        with patch.dict(os.environ, {}, clear=True):
            with patch("logging.basicConfig") as mock_cfg:
                m._setup_logging()
                _, kwargs = mock_cfg.call_args
                assert kwargs["level"] == logging.INFO

    def test_custom_log_level(self):
        m = self._import()
        with patch.dict(os.environ, {"LOG_LEVEL": "DEBUG"}, clear=True):
            with patch("logging.basicConfig") as mock_cfg:
                m._setup_logging()
                _, kwargs = mock_cfg.call_args
                assert kwargs["level"] == logging.DEBUG

    def test_json_format(self):
        m = self._import()
        with patch("logging.basicConfig") as mock_cfg:
            m._setup_logging()
            _, kwargs = mock_cfg.call_args
            assert kwargs["format"] is not None
            assert "%(message)s" in kwargs["format"]


class TestAddGenToPath:
    """Test _add_gen_to_path ensures generated stubs are importable."""

    def _import(self):
        for mod in list(sys.modules.keys()):
            if mod.startswith("hub_ai.__main__"):
                del sys.modules[mod]
        import hub_ai.__main__ as m
        return m

    def test_adds_gen_dir_to_sys_path(self):
        m = self._import()
        with patch("os.path.abspath", return_value="/fake/gen"), \
             patch("os.path.exists", return_value=True):
            original_path = list(sys.path)
            try:
                m._add_gen_to_path()
                assert "/fake/gen" in sys.path
            finally:
                sys.path = original_path


class TestStartNatsConsumers:
    """Test NATS consumer thread startup."""

    def _import(self):
        for mod in list(sys.modules.keys()):
            if mod.startswith("hub_ai.__main__"):
                del sys.modules[mod]
        import hub_ai.__main__ as m
        return m

    def test_starts_two_daemon_threads(self):
        m = self._import()
        mock_thread = MagicMock()
        with patch("threading.Thread", mock_thread):
            m._start_nats_consumers()
            assert mock_thread.call_count >= 2
            for call_args in mock_thread.call_args_list:
                _, kwargs = call_args
                assert kwargs.get("daemon") is True


class TestServeGrpc:
    """Test serve_grpc() — the async gRPC server setup."""

    @pytest.fixture(autouse=True)
    def _clean_imports(self):
        for mod in list(sys.modules.keys()):
            if mod.startswith("hub_ai._server") or mod.startswith("hub_ai.__main__"):
                del sys.modules[mod]
        yield

    def _build_serve_mocks(self):
        """Create the mock chain for serve_grpc imports."""
        mock_pb2_grpc = MagicMock()
        mock_server = AsyncMock()
        mock_server.add_insecure_port = MagicMock()
        mock_server.add_secure_port = MagicMock()
        _grpc_aio_mock.server.return_value = mock_server

        mock_servicer_cls = MagicMock()
        mock_servicer_instance = mock_servicer_cls.return_value

        return mock_pb2_grpc, mock_server, mock_servicer_cls, mock_servicer_instance

    @pytest.mark.asyncio
    async def test_creates_async_grpc_server(self):
        m = self._import()
        mock_pb2_grpc, mock_server, mock_servicer_cls, _ = self._build_serve_mocks()

        with patch.dict("sys.modules", {
            "nl2sql.v1.nl2sql_pb2_grpc": mock_pb2_grpc,
            "hub_ai._server": MagicMock(HubAIServicer=mock_servicer_cls),
        }), \
             patch.dict(os.environ, {}, clear=True):
            await m.serve_grpc()
            _grpc_aio_mock.server.assert_called_once()

    @pytest.mark.asyncio
    async def test_adds_servicer_to_server(self):
        m = self._import()
        mock_pb2_grpc, mock_server, mock_servicer_cls, mock_servicer_instance = self._build_serve_mocks()

        with patch.dict("sys.modules", {
            "nl2sql.v1.nl2sql_pb2_grpc": mock_pb2_grpc,
            "hub_ai._server": MagicMock(HubAIServicer=mock_servicer_cls),
        }), \
             patch.dict(os.environ, {}, clear=True):
            await m.serve_grpc()
            mock_pb2_grpc.add_NL2SQLServiceServicer_to_server.assert_called_once_with(
                mock_servicer_instance, mock_server
            )

    @pytest.mark.asyncio
    async def test_adds_insecure_port_by_default(self):
        m = self._import()
        mock_pb2_grpc, mock_server, mock_servicer_cls, _ = self._build_serve_mocks()

        with patch.dict("sys.modules", {
            "nl2sql.v1.nl2sql_pb2_grpc": mock_pb2_grpc,
            "hub_ai._server": MagicMock(HubAIServicer=mock_servicer_cls),
        }), \
             patch.dict(os.environ, {}, clear=True):
            await m.serve_grpc()
            mock_server.add_insecure_port.assert_called_once()

    @pytest.mark.asyncio
    async def test_uses_configured_addr(self):
        m = self._import()
        mock_pb2_grpc, mock_server, mock_servicer_cls, _ = self._build_serve_mocks()

        with patch.dict("sys.modules", {
            "nl2sql.v1.nl2sql_pb2_grpc": mock_pb2_grpc,
            "hub_ai._server": MagicMock(HubAIServicer=mock_servicer_cls),
        }), \
             patch.dict(os.environ, {"WORKER_GRPC_ADDR": "0.0.0.0:9999"}, clear=True):
            await m.serve_grpc()
            mock_server.add_insecure_port.assert_called_once_with("0.0.0.0:9999")

    @pytest.mark.asyncio
    async def test_starts_and_waits(self):
        m = self._import()
        mock_pb2_grpc, mock_server, mock_servicer_cls, _ = self._build_serve_mocks()

        with patch.dict("sys.modules", {
            "nl2sql.v1.nl2sql_pb2_grpc": mock_pb2_grpc,
            "hub_ai._server": MagicMock(HubAIServicer=mock_servicer_cls),
        }), \
             patch.dict(os.environ, {}, clear=True):
            await m.serve_grpc()
            mock_server.start.assert_called_once()
            mock_server.wait_for_termination.assert_called_once()

    @pytest.mark.asyncio
    async def test_enables_mtls_with_certs(self):
        m = self._import()
        mock_pb2_grpc, mock_server, mock_servicer_cls, _ = self._build_serve_mocks()

        mock_file = MagicMock()
        mock_file.__enter__.return_value.read.return_value = b"cert_bytes"

        with patch.dict("sys.modules", {
            "nl2sql.v1.nl2sql_pb2_grpc": mock_pb2_grpc,
            "hub_ai._server": MagicMock(HubAIServicer=mock_servicer_cls),
        }), \
             patch.dict(os.environ, {
                 "HUB_GRPC_CA_CERT": "/certs/ca.pem",
                 "HUB_GRPC_SERVER_CERT": "/certs/server.pem",
                 "HUB_GRPC_SERVER_KEY": "/certs/server.key",
             }, clear=True), \
             patch("builtins.open", return_value=mock_file):
            _grpc_mock.ssl_server_credentials = MagicMock(return_value="creds")
            await m.serve_grpc()
            mock_server.add_secure_port.assert_called_once()
            assert mock_server.add_insecure_port.call_count == 0

    def _import(self):
        for mod in list(sys.modules.keys()):
            if mod.startswith("hub_ai.__main__"):
                del sys.modules[mod]
        import hub_ai.__main__ as m
        return m
