import asyncio
from datetime import datetime
from uuid import uuid4

from fastapi import APIRouter, Depends, HTTPException, Query
from database import get_db
from models import (
    RegisterRequest, RegisterResponse,
    TaskResponse, TaskResultRequest, TaskResultResponse,
    KillResponse,
)
from auth import authenticate_agent
from config import settings

router = APIRouter(prefix="/api/v1/agent", tags=["agent"])

_tasks_events: dict[str, asyncio.Event] = {}
_events_lock = asyncio.Lock()


async def _get_event(machine_id: str) -> asyncio.Event:
    async with _events_lock:
        if machine_id not in _tasks_events:
            _tasks_events[machine_id] = asyncio.Event()
        return _tasks_events[machine_id]


async def notify_machine(machine_id: str):
    async with _events_lock:
        evt = _tasks_events.get(machine_id)
        if evt:
            evt.set()
            evt.clear()


@router.post("/register", response_model=RegisterResponse)
async def register(body: RegisterRequest):
    db = await get_db()
    machine_id = uuid4().hex[:12]

    try:
        await db.execute(
            "INSERT INTO machines (id, api_key, name, hostname, metadata) VALUES (?, ?, ?, ?, ?)",
            (machine_id, body.api_key, body.name, body.hostname or "", body.metadata or "{}"),
        )
        await db.commit()
    except Exception:
        raise HTTPException(status_code=409, detail="Registration conflict")

    return RegisterResponse(machine_id=machine_id, api_key=body.api_key)


@router.get("/next-task", response_model=TaskResponse)
async def next_task(
    wait: int = Query(default=settings.poll_timeout, ge=1, le=300),
    machine_id: str = Depends(authenticate_agent),
):
    db = await get_db()
    wait = min(wait, 300)

    row = await db.execute(
        "SELECT id, action, payload, timeout FROM tasks "
        "WHERE machine_id=? AND status='pending' "
        "ORDER BY created_at ASC LIMIT 1",
        (machine_id,),
    )
    task = await row.fetchone()

    if task:
        await db.execute(
            "UPDATE tasks SET status='dispatched' WHERE id=?",
            (task["id"],),
        )
        await db.commit()
        return TaskResponse(
            task={
                "task_id": task["id"],
                "action": task["action"],
                "payload": task["payload"],
                "timeout": task["timeout"],
            }
        )

    evt = await _get_event(machine_id)
    try:
        await asyncio.wait_for(evt.wait(), timeout=wait)
    except asyncio.TimeoutError:
        pass

    row = await db.execute(
        "SELECT id, action, payload, timeout FROM tasks "
        "WHERE machine_id=? AND status='pending' "
        "ORDER BY created_at ASC LIMIT 1",
        (machine_id,),
    )
    task = await row.fetchone()

    if task:
        await db.execute(
            "UPDATE tasks SET status='dispatched' WHERE id=?",
            (task["id"],),
        )
        await db.commit()
        return TaskResponse(
            task={
                "task_id": task["id"],
                "action": task["action"],
                "payload": task["payload"],
                "timeout": task["timeout"],
            }
        )

    return TaskResponse(task=None, message="no-op")


@router.post("/result", response_model=TaskResultResponse)
async def task_result(
    body: TaskResultRequest,
    machine_id: str = Depends(authenticate_agent),
):
    db = await get_db()

    row = await db.execute(
        "SELECT id FROM tasks WHERE id=? AND machine_id=?",
        (body.task_id, machine_id),
    )
    if not await row.fetchone():
        raise HTTPException(status_code=404, detail="Task not found")

    completed = datetime.utcnow().isoformat()
    await db.execute(
        "UPDATE tasks SET status=?, output=?, error=?, exit_code=?, completed_at=? WHERE id=?",
        (
            body.status,
            body.output,
            body.error,
            body.exit_code,
            completed,
            body.task_id,
        ),
    )
    await db.commit()
    return TaskResultResponse()


@router.get("/ping")
async def ping(machine_id: str = Depends(authenticate_agent)):
    return {"status": "ok", "machine_id": machine_id}
