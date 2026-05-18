"""Singleton HubInternalClient factory.

Provides a single shared HubInternalClient instance with async
connection lifecycle management. All callers should import and use
``get_client()`` instead of creating their own HubInternalClient.

Usage::

    from hub_ai._client import get_client, get_client_sync, close_client

    # In an async function:
    client = await get_client()
    await client.task_callback(...)

    # In a sync function:
    client = get_client_sync()
    client.task_callback_sync(...)

    # At shutdown:
    await close_client()
"""

from __future__ import annotations
import asyncio
import threading
import logging
from hub_ai.internal_client import HubInternalClient

logger = logging.getLogger(__name__)

_client_instance: HubInternalClient | None = None
_client_lock: asyncio.Lock = asyncio.Lock()
_client_lock_sync: threading.Lock = threading.Lock()


async def get_client() -> HubInternalClient:
    """Return the global HubInternalClient singleton (lazy init + async connect).

    Safe to call from async contexts (NATS consumers). The async gRPC
    channel (``_stub_aio``) is established on first call.
    """
    global _client_instance
    if _client_instance is None:
        async with _client_lock:
            if _client_instance is None:  # double-check
                _client_instance = HubInternalClient()
                await _client_instance._connect()
    else:
        # Ensure async channel is also connected if it was created
        # via get_client_sync first (which only sets up sync channel).
        if _client_instance._channel_aio is None:
            await _client_instance._connect()
    return _client_instance


def get_client_sync() -> HubInternalClient:
    """Return the global HubInternalClient singleton (synchronous).

    Safe to call from sync contexts (LangGraph node functions, thread pool).
    The sync gRPC channel is established in the constructor so no await is
    needed.
    """
    global _client_instance
    if _client_instance is None:
        with _client_lock_sync:
            if _client_instance is None:  # double-check
                _client_instance = HubInternalClient()
    return _client_instance


async def close_client() -> None:
    """Close the global HubInternalClient if it exists."""
    global _client_instance
    if _client_instance is not None:
        await _client_instance.close()
        _client_instance = None
        logger.info("HubInternalClient closed")
