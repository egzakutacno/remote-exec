from uuid import uuid4

from fastapi import APIRouter, Depends, HTTPException
from database import get_db
from models import (
    CreateTaskRequest, CreateTaskResponse,
    KillResponse,
    MachineListResponse, MachineInfo,
    HealthResponse,
)
from auth import authenticate_control
from command_validator import validate_task, ValidationError
from config import settings
import router_agent

router = APIRouter(prefix="/api/v1/control", tags=["control"])


@router.post("/tasks", response_model=CreateTaskResponse)
async def create_task(body: CreateTaskRequest, _: None = Depends(authenticate_control)):
    db = await get_db()

    row = await db.execute(
        "SELECT id FROM machines WHERE id=?", (body.machine_id,)
    )
    if not await row.fetchone():
        raise HTTPException(status_code=404, detail="Machine not found")

    try:
        validate_task(body.action, body.payload)
    except ValidationError as e:
        raise HTTPException(status_code=400, detail=str(e))

    task_id = uuid4().hex
    timeout = body.timeout or settings.default_task_timeout

    await db.execute(
        "INSERT INTO tasks (id, machine_id, action, payload, timeout) VALUES (?, ?, ?, ?, ?)",
        (task_id, body.machine_id, body.action, body.payload, timeout),
    )
    await db.commit()

    await router_agent.notify_machine(body.machine_id)

    return CreateTaskResponse(task_id=task_id)


@router.get("/tasks")
async def list_tasks(
    machine_id: str = None,
    status: str = None,
    limit: int = 50,
    _: None = Depends(authenticate_control),
):
    db = await get_db()
    query = "SELECT * FROM tasks WHERE 1=1"
    params = []

    if machine_id:
        query += " AND machine_id=?"
        params.append(machine_id)
    if status:
        query += " AND status=?"
        params.append(status)

    query += " ORDER BY created_at DESC LIMIT ?"
    params.append(limit)

    rows = await db.execute(query, params)
    tasks = [dict(r) for r in await rows.fetchall()]
    return {"tasks": tasks}


@router.get("/machines", response_model=MachineListResponse)
async def list_machines(_: None = Depends(authenticate_control)):
    db = await get_db()
    rows = await db.execute(
        "SELECT id, name, hostname, registered_at, last_seen, status FROM machines ORDER BY name"
    )
    machines = []
    for r in await rows.fetchall():
        machines.append(MachineInfo(
            id=r["id"], name=r["name"], hostname=r["hostname"] or "",
            registered_at=r["registered_at"], last_seen=r["last_seen"],
            status=r["status"],
        ))
    return MachineListResponse(machines=machines)


@router.post("/kill/{machine_id}", response_model=KillResponse)
async def kill_machine(machine_id: str, _: None = Depends(authenticate_control)):
    db = await get_db()
    row = await db.execute("SELECT id FROM machines WHERE id=?", (machine_id,))
    if not await row.fetchone():
        raise HTTPException(status_code=404, detail="Machine not found")

    task_id = uuid4().hex
    await db.execute(
        "INSERT INTO tasks (id, machine_id, action, payload, timeout) VALUES (?, ?, ?, ?, ?)",
        (task_id, machine_id, "kill", "", 5),
    )
    await db.commit()

    await router_agent.notify_machine(machine_id)

    return KillResponse(killed=True, machine_id=machine_id)


@router.get("/health", response_model=HealthResponse)
async def health(_: None = Depends(authenticate_control)):
    db = await get_db()
    mc = await db.execute("SELECT COUNT(*) FROM machines")
    machines_count = (await mc.fetchone())[0]
    tc = await db.execute("SELECT COUNT(*) FROM tasks WHERE status='pending'")
    pending = (await tc.fetchone())[0]
    return HealthResponse(machines=machines_count, pending_tasks=pending)
