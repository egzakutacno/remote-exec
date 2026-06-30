import json
import re
from config import settings


class ValidationError(Exception):
    pass


def validate_task(action: str, payload: str) -> dict:
    if action not in settings.allowed_actions:
        raise ValidationError(
            f"Action '{action}' not allowed. "
            f"Allowed: {', '.join(settings.allowed_actions)}"
        )

    if not payload and action in (
        "run_powershell", "run_cmd", "file_read",
        "file_write", "file_delete", "install_package",
    ):
        raise ValidationError(f"Action '{action}' requires a payload")

    if len(payload) > settings.max_payload_length:
        raise ValidationError(
            f"Payload too long ({len(payload)} chars). "
            f"Max: {settings.max_payload_length}"
        )

    if action == "run_powershell":
        _validate_powershell(payload)
    elif action == "run_cmd":
        _validate_cmd(payload)
    elif action == "restart_service":
        _validate_service(payload)
    elif action == "file_write":
        try:
            data = json.loads(payload)
            path = data.get("path", "")
        except json.JSONDecodeError:
            raise ValidationError("file_write payload must be JSON: {\"path\":...,\"content\":...}")
        _validate_file_path(action, path)
    elif action in ("file_read", "file_delete"):
        _validate_file_path(action, payload)
    elif action == "install_package":
        _validate_package(payload)

    if action == "run_powershell":
        validated = "powershell"
    elif action == "run_cmd":
        validated = "cmd"
    else:
        validated = action

    return {"action": action, "payload": payload, "validated_type": validated}


def _validate_powershell(payload: str):
    lower = payload.lower()
    for pattern in settings.powershell_denylist:
        if re.search(pattern, payload, re.IGNORECASE):
            raise ValidationError(
                f"PowerShell payload matches blocked pattern: {pattern}"
            )


def _validate_cmd(payload: str):
    for pattern in settings.cmd_denylist:
        if re.search(pattern, payload, re.IGNORECASE):
            raise ValidationError(f"CMD payload matches blocked pattern: {pattern}")


def _validate_service(service_name: str):
    allowed = {
        "hermes", "spooler", "wuauserv", "bits",
        "winrm", "w32time", "lanmanserver", "lanmanworkstation",
    }
    if service_name.lower() not in allowed:
        raise ValidationError(
            f"Service '{service_name}' not in allowed list. "
            f"Allowed: {', '.join(sorted(allowed))}"
        )


def _validate_file_path(action: str, path: str):
    normalized = path.replace("/", "\\").strip()
    check_list = (
        settings.allowed_read_paths
        if action in ("file_read", "file_delete")
        else settings.allowed_write_paths
    )
    ok = any(
        normalized.lower().startswith(p.lower()) for p in check_list
    )
    if not ok:
        raise ValidationError(
            f"Path '{path}' not in allowed directories: {check_list}"
        )


def _validate_package(package_name: str):
    if len(package_name) < 2:
        raise ValidationError("Package name too short")
    dangerous = {"winget", "choco", "scoop", "powershell", "python"}
    if package_name.lower() in dangerous:
        raise ValidationError(
            f"Cannot install meta-package manager: {package_name}"
        )
