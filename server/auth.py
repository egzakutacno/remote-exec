from fastapi import Depends, HTTPException, Security, Header
from fastapi.security import APIKeyHeader
from database import get_db
from config import settings

agent_auth = APIKeyHeader(name="X-API-Key", auto_error=False)
control_auth = APIKeyHeader(name="X-Control-Key", auto_error=False)


async def get_machine_id_from_header(
    x_machine_id: str = Header(None, alias="X-Machine-Id"),
) -> str:
    if not x_machine_id:
        raise HTTPException(status_code=400, detail="Missing X-Machine-Id header")
    return x_machine_id


async def authenticate_agent(
    machine_id: str = Depends(get_machine_id_from_header),
    api_key: str = Security(agent_auth),
) -> str:
    if not api_key:
        raise HTTPException(status_code=401, detail="Missing X-API-Key header")

    db = await get_db()
    row = await db.execute(
        "SELECT id FROM machines WHERE id=? AND api_key=?",
        (machine_id, api_key),
    )
    match = await row.fetchone()
    if not match:
        raise HTTPException(status_code=403, detail="Invalid machine_id or api_key")

    await db.execute(
        "UPDATE machines SET last_seen=datetime('now'), status='online' WHERE id=?",
        (machine_id,),
    )
    await db.commit()
    return machine_id


async def authenticate_control(api_key: str = Security(control_auth)) -> None:
    if not api_key:
        raise HTTPException(status_code=401, detail="Missing X-Control-Key header")
    if api_key != settings.control_api_key:
        raise HTTPException(status_code=403, detail="Invalid control API key")
