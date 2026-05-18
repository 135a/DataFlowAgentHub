"""Tracing helpers for reporting LangGraph step progress to the Go backend.

``report_run_step`` is a synchronous convenience that obtains the global
HubInternalClient singleton and calls ``run_step_callback_sync``.
"""

from __future__ import annotations
import logging

logger = logging.getLogger(__name__)


def report_run_step(
    run_id: str,
    agent_name: str,
    status: str,
    input_summary: str = "",
    output_summary: str = "",
    error_message: str = "",
) -> None:
    """Report a LangGraph agent step to Go API (synchronous).

    Uses the sync gRPC stub (created at HubInternalClient construction)
    so this function is safe to call from LangGraph sync node functions.
    """
    try:
        from hub_ai._client import get_client_sync
        client = get_client_sync()
        client.run_step_callback_sync(
            run_id=run_id,
            agent_name=agent_name,
            status=status,
            input_summary=input_summary,
            output_summary=output_summary,
            error_message=error_message,
        )
    except Exception as e:
        logger.warning("Failed to report run step via gRPC: %s", e)
