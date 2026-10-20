#!/usr/bin/env python3
"""Create deterministic Mattermost load-test users and write an ltagent credential file.

Secrets are read from environment variables and are never written to the result log.
The credential output intentionally contains plaintext passwords because that is the
format required by mattermost-load-test-ng; protect it with mode 0600.
"""

from __future__ import annotations

import argparse
import concurrent.futures
import json
import os
import random
import ssl
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, asdict
from pathlib import Path


@dataclass(frozen=True)
class Result:
    index: int
    username: str
    email: str
    status: str
    user_id: str = ""
    detail: str = ""


class MattermostClient:
    def __init__(
        self,
        server: str,
        token: str,
        password: str,
        timeout: float,
        retries: int,
        request_delay: float,
        insecure: bool,
    ) -> None:
        self.server = server.rstrip("/")
        self.token = token
        self.password = password
        self.timeout = timeout
        self.retries = retries
        self.request_delay = request_delay
        self.ssl_context = ssl._create_unverified_context() if insecure else None
        self._throttle_lock = threading.Lock()
        self._last_request_at = 0.0

    def _throttle(self) -> None:
        if self.request_delay <= 0:
            return
        with self._throttle_lock:
            now = time.monotonic()
            wait_for = self.request_delay - (now - self._last_request_at)
            if wait_for > 0:
                time.sleep(wait_for)
            self._last_request_at = time.monotonic()

    def request(
        self,
        method: str,
        path: str,
        body: dict | None = None,
        use_admin_token: bool = True,
    ) -> tuple[int, dict | list | str]:
        url = f"{self.server}{path}"
        payload = None if body is None else json.dumps(body).encode("utf-8")
        headers = {"Accept": "application/json"}
        if body is not None:
            headers["Content-Type"] = "application/json"
        if use_admin_token:
            headers["Authorization"] = f"Bearer {self.token}"

        last_error = ""
        for attempt in range(self.retries + 1):
            self._throttle()
            req = urllib.request.Request(url, data=payload, headers=headers, method=method)
            try:
                with urllib.request.urlopen(
                    req, timeout=self.timeout, context=self.ssl_context
                ) as response:
                    raw = response.read().decode("utf-8", errors="replace")
                    return response.status, json.loads(raw) if raw else {}
            except urllib.error.HTTPError as exc:
                raw = exc.read().decode("utf-8", errors="replace")
                try:
                    parsed: dict | list | str = json.loads(raw) if raw else {}
                except json.JSONDecodeError:
                    parsed = raw[:500]
                if exc.code not in {429, 500, 502, 503, 504} or attempt >= self.retries:
                    return exc.code, parsed
                last_error = f"HTTP {exc.code}: {parsed}"
            except (urllib.error.URLError, TimeoutError, OSError) as exc:
                last_error = str(exc)
                if attempt >= self.retries:
                    return 0, last_error

            time.sleep(min(2**attempt + random.random(), 15))

        return 0, last_error or "unknown request error"

    def verify_admin(self) -> None:
        status, body = self.request("GET", "/api/v4/users/me")
        if status != 200 or not isinstance(body, dict):
            raise RuntimeError(f"admin token validation failed: HTTP {status}, {body}")
        roles = str(body.get("roles", ""))
        if "system_admin" not in roles.split():
            raise RuntimeError("MM_TOKEN does not belong to a system_admin user")

    def verify_login(self, username: str) -> bool:
        status, _ = self.request(
            "POST",
            "/api/v4/users/login",
            {"login_id": username, "password": self.password},
            use_admin_token=False,
        )
        return status == 200

    def ensure_user(self, index: int, username: str, email: str) -> Result:
        quoted = urllib.parse.quote(username, safe="")
        status, existing = self.request("GET", f"/api/v4/users/username/{quoted}")
        if status == 200 and isinstance(existing, dict):
            if self.verify_login(username):
                return Result(
                    index, username, email, "existing_verified", str(existing.get("id", ""))
                )
            return Result(
                index,
                username,
                email,
                "conflict",
                str(existing.get("id", "")),
                "username exists but configured password cannot log in",
            )
        if status != 404:
            return Result(index, username, email, "lookup_failed", detail=f"HTTP {status}: {existing}")

        create_body = {
            "username": username,
            "email": email,
            "password": self.password,
            "email_verified": True,
            "disable_welcome_email": True,
        }
        status, created = self.request("POST", "/api/v4/users", create_body)
        if status in {200, 201} and isinstance(created, dict):
            return Result(
                index, username, email, "created", str(created.get("id", ""))
            )

        # An interrupted earlier run may race with this run. Re-check and verify.
        status2, existing2 = self.request("GET", f"/api/v4/users/username/{quoted}")
        if status2 == 200 and isinstance(existing2, dict) and self.verify_login(username):
            return Result(
                index,
                username,
                email,
                "existing_verified",
                str(existing2.get("id", "")),
            )

        return Result(index, username, email, "create_failed", detail=f"HTTP {status}: {created}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Create Mattermost load-test users and an ltagent credential file."
    )
    parser.add_argument("--server", required=True, help="Mattermost base URL")
    parser.add_argument("--count", type=int, default=12000)
    parser.add_argument("--start-index", type=int, default=1)
    parser.add_argument("--prefix", default="imlt")
    parser.add_argument("--email-domain", default="loadtest.example.com")
    parser.add_argument("--workers", type=int, default=5)
    parser.add_argument("--request-delay", type=float, default=0.05)
    parser.add_argument("--timeout", type=float, default=15)
    parser.add_argument("--retries", type=int, default=4)
    parser.add_argument("--output", required=True, help="ltagent credential file")
    parser.add_argument("--result-log", required=True, help="JSON Lines result file")
    parser.add_argument("--token-env", default="MM_TOKEN")
    parser.add_argument("--password-env", default="IMLT_USER_PASSWORD")
    parser.add_argument(
        "--insecure",
        action="store_true",
        help="Disable TLS certificate verification; use only in an isolated test environment",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.count <= 0 or args.start_index <= 0 or args.workers <= 0:
        print("count, start-index and workers must be positive", file=sys.stderr)
        return 2

    token = os.environ.get(args.token_env, "")
    password = os.environ.get(args.password_env, "")
    if not token:
        print(f"missing environment variable: {args.token_env}", file=sys.stderr)
        return 2
    if len(password) < 10:
        print(
            f"{args.password_env} must contain a test-only password of at least 10 characters",
            file=sys.stderr,
        )
        return 2

    output = Path(args.output).expanduser().resolve()
    result_log = Path(args.result_log).expanduser().resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    result_log.parent.mkdir(parents=True, exist_ok=True)

    client = MattermostClient(
        server=args.server,
        token=token,
        password=password,
        timeout=args.timeout,
        retries=args.retries,
        request_delay=args.request_delay,
        insecure=args.insecure,
    )
    client.verify_admin()

    planned: list[tuple[int, str, str]] = []
    end = args.start_index + args.count
    width = max(5, len(str(end - 1)))
    for index in range(args.start_index, end):
        username = f"{args.prefix}{index:0{width}d}"
        email = f"{username}@{args.email_domain}"
        planned.append((index, username, email))

    results: list[Result] = []
    with result_log.open("w", encoding="utf-8") as log_handle:
        with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
            futures = {
                pool.submit(client.ensure_user, index, username, email): index
                for index, username, email in planned
            }
            total = len(futures)
            for completed, future in enumerate(
                concurrent.futures.as_completed(futures), start=1
            ):
                index = futures[future]
                try:
                    result = future.result()
                except Exception as exc:  # preserve per-user failure and continue
                    username = next(item[1] for item in planned if item[0] == index)
                    email = next(item[2] for item in planned if item[0] == index)
                    result = Result(index, username, email, "exception", detail=str(exc))
                results.append(result)
                log_handle.write(json.dumps(asdict(result), ensure_ascii=False) + "\n")
                log_handle.flush()
                if completed % 100 == 0 or completed == total:
                    print(f"completed={completed}/{total}", flush=True)

    usable_statuses = {"created", "existing_verified"}
    usable = sorted((r for r in results if r.status in usable_statuses), key=lambda r: r.index)
    failed = [r for r in results if r.status not in usable_statuses]

    with output.open("w", encoding="utf-8", newline="\n") as credential_handle:
        for result in usable:
            credential_handle.write(f"{result.email} {password}\n")
    os.chmod(output, 0o600)
    os.chmod(result_log, 0o600)

    counts: dict[str, int] = {}
    for result in results:
        counts[result.status] = counts.get(result.status, 0) + 1
    print(json.dumps({"planned": len(results), "usable": len(usable), "failed": len(failed), "status_counts": counts}, ensure_ascii=False, indent=2))
    print(f"credential_file={output}")
    print(f"result_log={result_log}")
    return 0 if len(usable) == args.count and not failed else 1


if __name__ == "__main__":
    raise SystemExit(main())
