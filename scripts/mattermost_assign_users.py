#!/usr/bin/env python3
"""Assign Mattermost load-test users to one team and one channel.

Input format is the mattermost-load-test-ng credential format:

    username password

The password is never written to the result log. Admin authentication is read
from MM_TOKEN (or --token-env). The script is idempotent: memberships that
already exist are verified and retained.
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
from dataclasses import asdict, dataclass
from pathlib import Path


@dataclass(frozen=True)
class Result:
    index: int
    username: str
    user_id: str
    status: str
    team_status: str = ""
    channel_status: str = ""
    detail: str = ""


class MattermostClient:
    def __init__(
        self,
        server: str,
        token: str,
        timeout: float,
        retries: int,
        request_delay: float,
        insecure: bool,
    ) -> None:
        self.server = server.rstrip("/")
        self.token = token
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
        self, method: str, path: str, body: dict | None = None
    ) -> tuple[int, dict | list | str]:
        url = f"{self.server}{path}"
        payload = None if body is None else json.dumps(body).encode("utf-8")
        headers = {
            "Accept": "application/json",
            "Authorization": f"Bearer {self.token}",
        }
        if body is not None:
            headers["Content-Type"] = "application/json"

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
        if "system_admin" not in str(body.get("roles", "")).split():
            raise RuntimeError("MM_TOKEN does not belong to a system_admin user")

    def verify_targets(self, team_id: str, channel_id: str) -> None:
        status, team = self.request("GET", f"/api/v4/teams/{team_id}")
        if status != 200 or not isinstance(team, dict):
            raise RuntimeError(f"team validation failed: HTTP {status}, {team}")
        status, channel = self.request("GET", f"/api/v4/channels/{channel_id}")
        if status != 200 or not isinstance(channel, dict):
            raise RuntimeError(f"channel validation failed: HTTP {status}, {channel}")
        if str(channel.get("team_id", "")) != team_id:
            raise RuntimeError("channel does not belong to the selected team")

    def ensure_memberships(
        self, index: int, username: str, team_id: str, channel_id: str
    ) -> Result:
        quoted = urllib.parse.quote(username, safe="")
        status, user = self.request("GET", f"/api/v4/users/email/{quoted}")
        if status != 200 or not isinstance(user, dict) or not user.get("id"):
            return Result(
                index, username, "", "user_lookup_failed", detail=f"HTTP {status}: {user}"
            )
        user_id = str(user["id"])

        team_status, team_body = self.request(
            "GET", f"/api/v4/teams/{team_id}/members/{user_id}"
        )
        if team_status == 200:
            team_result = "existing"
        elif team_status == 404:
            team_status, team_body = self.request(
                "POST",
                f"/api/v4/teams/{team_id}/members",
                {"team_id": team_id, "user_id": user_id},
            )
            if team_status not in {200, 201}:
                return Result(
                    index,
                    username,
                    user_id,
                    "team_add_failed",
                    detail=f"HTTP {team_status}: {team_body}",
                )
            team_result = "added"
        else:
            return Result(
                index,
                username,
                user_id,
                "team_check_failed",
                detail=f"HTTP {team_status}: {team_body}",
            )

        channel_status, channel_body = self.request(
            "GET", f"/api/v4/channels/{channel_id}/members/{user_id}"
        )
        if channel_status == 200:
            channel_result = "existing"
        elif channel_status == 404:
            channel_status, channel_body = self.request(
                "POST",
                f"/api/v4/channels/{channel_id}/members",
                {"user_id": user_id},
            )
            if channel_status not in {200, 201}:
                return Result(
                    index,
                    username,
                    user_id,
                    "channel_add_failed",
                    team_status=team_result,
                    detail=f"HTTP {channel_status}: {channel_body}",
                )
            channel_result = "added"
        else:
            return Result(
                index,
                username,
                user_id,
                "channel_check_failed",
                team_status=team_result,
                detail=f"HTTP {channel_status}: {channel_body}",
            )

        # Final verification prevents a successful POST from being treated as sufficient.
        verify_team, _ = self.request(
            "GET", f"/api/v4/teams/{team_id}/members/{user_id}"
        )
        verify_channel, _ = self.request(
            "GET", f"/api/v4/channels/{channel_id}/members/{user_id}"
        )
        if verify_team != 200 or verify_channel != 200:
            return Result(
                index,
                username,
                user_id,
                "verification_failed",
                team_status=team_result,
                channel_status=channel_result,
                detail=f"team_http={verify_team}, channel_http={verify_channel}",
            )

        return Result(
            index,
            username,
            user_id,
            "ready",
            team_status=team_result,
            channel_status=channel_result,
        )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Assign Mattermost load-test users to a team and channel."
    )
    parser.add_argument("--server", required=True)
    parser.add_argument("--users-file", required=True)
    parser.add_argument("--team-id", required=True)
    parser.add_argument("--channel-id", required=True)
    parser.add_argument("--workers", type=int, default=8)
    parser.add_argument("--request-delay", type=float, default=0.03)
    parser.add_argument("--timeout", type=float, default=15)
    parser.add_argument("--retries", type=int, default=4)
    parser.add_argument("--result-log", required=True)
    parser.add_argument("--token-env", default="MM_TOKEN")
    parser.add_argument("--insecure", action="store_true")
    return parser.parse_args()


def read_users(path: Path) -> list[tuple[int, str]]:
    users: list[tuple[int, str]] = []
    seen: set[str] = set()
    with path.open("r", encoding="utf-8") as handle:
        for line_number, raw in enumerate(handle, start=1):
            line = raw.strip()
            if not line:
                continue
            fields = line.split()
            if len(fields) != 2:
                raise ValueError(
                    f"invalid credential format at line {line_number}: expected 2 fields"
                )
            username = fields[0]
            if username in seen:
                raise ValueError(f"duplicate username at line {line_number}: {username}")
            seen.add(username)
            users.append((line_number, username))
    if not users:
        raise ValueError("credential file contains no users")
    return users


def main() -> int:
    args = parse_args()
    if args.workers <= 0 or args.timeout <= 0 or args.retries < 0:
        print("workers/timeout must be positive and retries must be non-negative", file=sys.stderr)
        return 2
    token = os.environ.get(args.token_env, "")
    if not token:
        print(f"missing environment variable: {args.token_env}", file=sys.stderr)
        return 2

    users_file = Path(args.users_file).expanduser().resolve()
    result_log = Path(args.result_log).expanduser().resolve()
    if not users_file.is_file():
        print(f"users file not found: {users_file}", file=sys.stderr)
        return 2
    result_log.parent.mkdir(parents=True, exist_ok=True)

    try:
        users = read_users(users_file)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 2

    client = MattermostClient(
        server=args.server,
        token=token,
        timeout=args.timeout,
        retries=args.retries,
        request_delay=args.request_delay,
        insecure=args.insecure,
    )
    try:
        client.verify_admin()
        client.verify_targets(args.team_id, args.channel_id)
    except RuntimeError as exc:
        print(str(exc), file=sys.stderr)
        return 2

    results: list[Result] = []
    with result_log.open("w", encoding="utf-8", newline="\n") as log_handle:
        with concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as pool:
            futures = {
                pool.submit(
                    client.ensure_memberships,
                    index,
                    username,
                    args.team_id,
                    args.channel_id,
                ): (index, username)
                for index, username in users
            }
            total = len(futures)
            for completed, future in enumerate(
                concurrent.futures.as_completed(futures), start=1
            ):
                index, username = futures[future]
                try:
                    result = future.result()
                except Exception as exc:
                    result = Result(index, username, "", "exception", detail=str(exc))
                results.append(result)
                log_handle.write(json.dumps(asdict(result), ensure_ascii=False) + "\n")
                log_handle.flush()
                if completed % 100 == 0 or completed == total:
                    print(f"completed={completed}/{total}", flush=True)

    os.chmod(result_log, 0o600)
    counts: dict[str, int] = {}
    for result in results:
        counts[result.status] = counts.get(result.status, 0) + 1
    ready = counts.get("ready", 0)
    summary = {
        "planned": len(results),
        "ready": ready,
        "failed": len(results) - ready,
        "status_counts": counts,
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    print(f"result_log={result_log}")
    return 0 if ready == len(results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
