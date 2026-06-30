from pydantic import BaseModel, Field
from typing import Optional
from uuid import uuid4


def new_uuid() -> str:
    return uuid4().hex


class RegisterRequest(BaseModel):
    name: str
    api_key: str = Field(default_factory=new_uuid)
    hostname: str = ""
    metadata: str = "{}"


class RegisterResponse(BaseModel):
    machine_id: str
    api_key: str
    status: str = "registered"


class TaskRequest(BaseModel):
    action: str
    payload: str = ""
    timeout: int = 30
    machine_id: Optional[str] = None


class TaskResponse(BaseModel):
    task: Optional[dict] = None
    message: str = ""


class TaskResultRequest(BaseModel):
    task_id: str
    status: str
    output: str = ""
    error: str = ""
    exit_code: Optional[int] = None


class TaskResultResponse(BaseModel):
    ok: bool = True


class CreateTaskRequest(BaseModel):
    machine_id: str
    action: str
    payload: str = ""
    timeout: int = 30


class CreateTaskResponse(BaseModel):
    task_id: str
    status: str = "created"


class KillRequest(BaseModel):
    pass


class KillResponse(BaseModel):
    killed: bool
    machine_id: str


class MachineInfo(BaseModel):
    id: str
    name: str
    hostname: str
    registered_at: str
    last_seen: Optional[str]
    status: str


class MachineListResponse(BaseModel):
    machines: list[MachineInfo]


class HealthResponse(BaseModel):
    status: str = "ok"
    machines: int = 0
    pending_tasks: int = 0
