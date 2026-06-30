import aiosqlite
import asyncio
import os
from config import settings

_db = None
_ready = asyncio.Event()


async def get_db() -> aiosqlite.Connection:
    await _ready.wait()
    return _db


async def init_db():
    global _db
    os.makedirs(os.path.dirname(settings.db_path), exist_ok=True)

    _db = await aiosqlite.connect(settings.db_path)
    _db.row_factory = aiosqlite.Row
    await _db.execute("PRAGMA journal_mode=WAL")
    await _db.execute("PRAGMA foreign_keys=ON")

    await _db.execute("""
        CREATE TABLE IF NOT EXISTS machines (
            id TEXT PRIMARY KEY,
            api_key TEXT NOT NULL UNIQUE,
            name TEXT NOT NULL,
            hostname TEXT DEFAULT '',
            registered_at TEXT NOT NULL DEFAULT (datetime('now')),
            last_seen TEXT,
            status TEXT DEFAULT 'offline',
            metadata TEXT DEFAULT '{}'
        )
    """)

    await _db.execute("""
        CREATE TABLE IF NOT EXISTS tasks (
            id TEXT PRIMARY KEY,
            machine_id TEXT NOT NULL,
            action TEXT NOT NULL,
            payload TEXT NOT NULL DEFAULT '',
            timeout INTEGER DEFAULT 30,
            status TEXT DEFAULT 'pending',
            created_at TEXT NOT NULL DEFAULT (datetime('now')),
            completed_at TEXT,
            output TEXT,
            error TEXT,
            exit_code INTEGER,
            FOREIGN KEY (machine_id) REFERENCES machines(id)
        )
    """)

    await _db.execute("""
        CREATE INDEX IF NOT EXISTS idx_tasks_machine_status
        ON tasks(machine_id, status)
    """)

    await _db.commit()
    _ready.set()


async def close_db():
    global _db
    if _db:
        await _db.close()
        _db = None
