import os
from dataclasses import dataclass, field


@dataclass
class Settings:
    host: str = os.getenv("REMOTE_EXEC_HOST", "0.0.0.0")
    port: int = int(os.getenv("REMOTE_EXEC_PORT", "9990"))
    db_path: str = os.getenv("REMOTE_EXEC_DB", "/var/lib/remote-exec/data/remote_exec.db")

    poll_timeout: int = 60
    poll_check_interval: float = 1.0

    default_task_timeout: int = 30

    control_api_key: str = os.getenv(
        "REMOTE_EXEC_CONTROL_KEY",
        "control-key-change-me"
    )

    allowed_actions: list = field(default_factory=lambda: [
        "ping",
        "run_powershell",
        "run_cmd",
        "restart_service",
        "file_read",
        "file_write",
        "file_delete",
        "install_package",
        "kill",
    ])

    powershell_denylist: list = field(default_factory=lambda: [
        r"(?i)remove-item\s+-recurse",
        r"(?i)format-volume",
        r"(?i)clear-disk",
        r"(?i)stop-computer",
        r"(?i)restart-computer",
        r"(?i)shutdown",
    ])

    cmd_denylist: list = field(default_factory=lambda: [
        r"(?i)format\s",
        r"(?i)diskpart",
        r"(?i)shutdown",
        r"(?i)del\s+/[fF]\s+/[sS]",
    ])

    allowed_read_paths: list = field(default_factory=lambda: [
        "C:\\Users",
        "C:\\ProgramData",
        "D:\\",
        "Z:\\",
    ])

    allowed_write_paths: list = field(default_factory=lambda: [
        "C:\\Users",
        "D:\\",
        "Z:\\",
    ])

    max_payload_length: int = 100_000
    max_file_write_size: int = 10 * 1024 * 1024

    log_level: str = "INFO"


settings = Settings()
