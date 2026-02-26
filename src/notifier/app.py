"""
notifier – a lightweight FastAPI service used to demonstrate
OpenTelemetry zero-code auto-instrumentation in the workshop.

This file contains NO OpenTelemetry imports or manual SDK calls.
Traces, metrics, and logs are produced entirely by the OTel Operator
injecting opentelemetry-distro via an init-container at pod start.

Endpoints:
  POST /notify   – accept a note lifecycle event and record it in the
                   database service.
  GET  /healthz  – liveness / readiness probe.
"""

import os

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI(title="notifier")

DATABASE_URL = os.environ.get("DATABASE_API_URL", "http://database:8082").rstrip("/")
SERVICE_NAME = os.environ.get("SERVICE_NAME", "notifier")


class NotifyRequest(BaseModel):
    action: str  # "created" | "updated" | "deleted"
    title: str = ""
    note_id: int = 0


@app.get("/healthz")
def health() -> dict:
    return {"status": "ok", "service": SERVICE_NAME}


@app.post("/notify")
def notify(req: NotifyRequest) -> dict:
    """
    Record a note lifecycle event in the database service.

    The database call is made synchronously so that the full call chain
    (backend → notifier → database) appears in the distributed trace once
    auto-instrumentation is enabled via the OTel Operator.
    """
    label = req.title if req.title else f"note_id={req.note_id}"
    message = f"note {req.action}: {label}"

    try:
        with httpx.Client(timeout=5.0) as client:
            resp = client.post(
                f"{DATABASE_URL}/events",
                json={
                    "source": SERVICE_NAME,
                    "method": "POST",
                    "route": "/notify",
                    "status": 200,
                    "message": message,
                },
            )
            resp.raise_for_status()
    except httpx.HTTPError as exc:
        raise HTTPException(status_code=502, detail=str(exc)) from exc

    return {"status": "ok", "service": SERVICE_NAME, "action": req.action}
