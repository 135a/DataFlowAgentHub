"""Entry-point for the Hub AI Worker.

Starts the async gRPC server and background NATS consumer threads.
"""

from __future__ import annotations
import asyncio
import logging
import os
import sys

import grpc
from grpc import aio as grpc_aio

_WORKER_VERSION = "0.1.0"
logger = logging.getLogger(__name__)


def _setup_logging():
    level_name = os.environ.get("LOG_LEVEL", "INFO")
    level = getattr(logging, level_name.upper(), logging.INFO)
    logging.basicConfig(
        level=level,
        format='{"level":"%(levelname)s","msg":"%(message)s","logger":"%(name)s"}',
    )


def _add_gen_to_path():
    """Ensure generated protobuf stubs are importable."""
    base = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "gen"))
    if base not in sys.path:
        sys.path.insert(0, base)


async def serve_grpc() -> None:
    """Build and start the async gRPC server, then block forever."""
    from nl2sql.v1 import nl2sql_pb2_grpc
    from hub_ai._server import HubAIServicer

    server: grpc_aio.Server = grpc.aio.server()
    nl2sql_pb2_grpc.add_NL2SQLServiceServicer_to_server(HubAIServicer(), server)

    addr = os.environ.get("WORKER_GRPC_ADDR", "0.0.0.0:50051")

    # mTLS support
    ca_cert = os.environ.get("HUB_GRPC_CA_CERT") or ""
    server_cert = os.environ.get("HUB_GRPC_SERVER_CERT") or ""
    server_key = os.environ.get("HUB_GRPC_SERVER_KEY") or ""
    if ca_cert and server_cert and server_key:
        with open(server_cert, "rb") as f:
            server_cert_bytes = f.read()
        with open(server_key, "rb") as f:
            server_key_bytes = f.read()
        with open(ca_cert, "rb") as f:
            ca_bytes = f.read()
        creds = grpc.ssl_server_credentials(
            [(server_key_bytes, server_cert_bytes)],
            root_certificates=ca_bytes,
            require_client_auth=True,
        )
        server.add_secure_port(addr, creds)
        logger.info("grpc mTLS enabled on %s", addr)
    else:
        server.add_insecure_port(addr)
        logger.info("grpc insecure on %s", addr)

    await server.start()
    logger.info("grpc listening on %s", addr)
    await server.wait_for_termination()


def _start_agent_consumer():
    """Run the NATS agent-pipeline consumer in a daemon thread."""
    from orchestrator.consumer import run_consumer

    try:
        asyncio.run(run_consumer())
    except Exception as e:
        logger.error("Agent consumer died: %s", e)


def _start_knowledge_consumer():
    """Run the NATS knowledge-doc consumer in a daemon thread."""
    from orchestrator.knowledge_consumer import run_knowledge_consumer

    try:
        asyncio.run(run_knowledge_consumer())
    except Exception as e:
        logger.error("Knowledge consumer died: %s", e)


def _start_nats_consumers():
    """Start both NATS consumers as daemon threads."""
    import threading

    threads = [
        threading.Thread(target=_start_agent_consumer, daemon=True),
        threading.Thread(target=_start_knowledge_consumer, daemon=True),
    ]
    for t in threads:
        t.start()
    logger.info("Started NATS consumer threads (agent + knowledge)")


def main() -> None:
    _setup_logging()
    _add_gen_to_path()

    # Verify generated stubs are available
    try:
        from nl2sql.v1 import nl2sql_pb2  # noqa: F401
    except ImportError:
        logger.error("missing generated stubs; run `make gen-py` or build Docker image")
        sys.exit(1)

    _start_nats_consumers()

    try:
        asyncio.run(serve_grpc())
    except KeyboardInterrupt:
        logger.info("shutting down")


if __name__ == "__main__":
    main()
