#!/usr/bin/env python3
"""Apply TogoAPI login-agreement documents through Core's admin settings API."""

from __future__ import annotations

import json
import os
from pathlib import Path
from urllib.request import Request, urlopen


DEPLOY_ROOT = Path(__file__).resolve().parent
LEGAL_ROOT = Path(
    os.environ.get("TOGO_LEGAL_ROOT", DEPLOY_ROOT.parent / "docs" / "legal")
)
CORE_ENV_PATH = Path(os.environ.get("TOGO_CORE_ENV", "/opt/portal/.env"))
CORE_SETTINGS_URL = os.environ.get(
    "TOGO_CORE_SETTINGS_URL",
    "http://127.0.0.1:8080/api/v1/admin/settings",
)


def read_dotenv_value(path: Path, key: str) -> str:
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        name, value = line.split("=", 1)
        if name.strip() == key:
            return value.strip().strip("\"'")
    raise RuntimeError(f"Missing {key} in {path}")


def read_document(document_id: str, title: str, filename: str) -> dict[str, str]:
    content = (LEGAL_ROOT / filename).read_text(encoding="utf-8").strip()
    if not content:
        raise RuntimeError(f"Legal document is empty: {filename}")
    return {"id": document_id, "title": title, "content_md": content}


def main() -> None:
    admin_api_key = read_dotenv_value(CORE_ENV_PATH, "CORE_ADMIN_API_KEY")
    payload = {
        "login_agreement_enabled": True,
        "login_agreement_mode": "checkbox",
        "login_agreement_updated_at": "2026-08-01",
        "login_agreement_documents": [
            read_document("terms", "服务条款", "terms.zh.md"),
            read_document("usage-policy", "使用政策", "usage-policy.zh.md"),
            read_document(
                "supported-regions",
                "支持的国家和地区",
                "supported-regions.zh.md",
            ),
            read_document("privacy-policy", "隐私政策", "privacy-policy.zh.md"),
        ],
    }
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = Request(
        CORE_SETTINGS_URL,
        data=body,
        method="PUT",
        headers={
            "Content-Type": "application/json; charset=utf-8",
            "x-api-key": admin_api_key,
        },
    )
    with urlopen(request, timeout=30) as response:
        response_body = response.read().decode("utf-8", errors="replace")
        if response.status < 200 or response.status >= 300:
            raise RuntimeError(f"Settings update failed ({response.status}): {response_body}")
        print("Login agreement settings updated successfully.")


if __name__ == "__main__":
    main()
