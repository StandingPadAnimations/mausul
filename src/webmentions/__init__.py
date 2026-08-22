# Copyright (C) Maryam Stellamaris (Mahid Sheikh)
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.

import asyncio
from contextlib import asynccontextmanager
from collections.abc import AsyncGenerator
import html
import ipaddress
import os
import signal
import time
import socket
from typing import Annotated, final
from urllib.parse import urldefrag, urljoin, urlparse

import asqlite
from fastapi import (
    BackgroundTasks,
    FastAPI,
    Form,
    HTTPException,
    Query,
    Request,
    status,
)
from fastapi.exceptions import RequestValidationError
from fastapi.responses import HTMLResponse, JSONResponse
import httpx
from pydantic import HttpUrl
from selectolax.parser import HTMLParser
from slowapi import Limiter, _rate_limit_exceeded_handler
from slowapi.errors import RateLimitExceeded
from slowapi.util import get_remote_address
from starlette.middleware.base import BaseHTTPMiddleware

# Limit of 1 MB to avoid memory issues
MAX_RESPONSE_SIZE = 1_048_576
FETCH_TIMEOUT = 5.0
DB_FILE = "/webmentions/webmentions.db"
ALLOWED_TARGET_DOMAINS = {
    "standingpad.org",
    "www.standingpad.org",
    "localhost",
    "127.0.0.1",
}
ALLOWED_PATH_PREFIX = "/posts/"

# Automatic idle shutdown
IDLE_TIMEOUT_SECONDS = 300

db_pool: asqlite.Pool | None = None

# Middleware to shut down on idle,
# since it is intended to run this
# program with SystemD Socket Activaion
@final
class IdleTimeoutMiddleware(BaseHTTPMiddleware):
    def __init__(self, app: FastAPI):
        super().__init__(app)
        self.last_activity = time.time()
        _ = asyncio.create_task(self._watchdog())

    async def dispatch(self, request: Request, call_next):
        self.last_activity = time.time()
        return await call_next(request)

    async def _watchdog(self):
        while True:
            await asyncio.sleep(15)
            if time.time() - self.last_activity > IDLE_TIMEOUT_SECONDS:
                os.kill(os.getpid(), signal.SIGTERM)
                break

async def init_db() -> None:
    assert db_pool is not None
    async with db_pool.acquire() as conn:
        _ = await conn.execute(
            """
            CREATE TABLE IF NOT EXISTS webmentions (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                source TEXT NOT NULL,
                target TEXT NOT NULL,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                UNIQUE(source, target)
            );
            """
        )
        _ = await conn.execute(
            "CREATE INDEX IF NOT EXISTS idx_source ON webmentions(target)"
        )
        await conn.commit()


@asynccontextmanager
async def lifespan(_app: FastAPI) -> AsyncGenerator[None, None]:
    global db_pool
    async with asqlite.create_pool(DB_FILE) as pool:
        db_pool = pool
        await init_db()
        yield
    db_pool = None


def is_allowed_url(url: str) -> bool:
    """Check if the URL is the proper protocol, with a hostname, and is on a public IP address."""
    parsed = urlparse(url)
    if parsed.scheme not in ("http", "https"):
        return False

    hostname = parsed.hostname
    if not hostname:
        return False

    try:
        ip_str = socket.gethostbyname(hostname)
        ip = ipaddress.ip_address(ip_str)
        if ip.is_private or ip.is_loopback or ip.is_link_local or ip.is_reserved:
            return False
    except socket.gaierror, ValueError:
        return False
    return True


def is_allowed_target(url: str) -> bool:
    """Check if the URL is allowed to be a target for a Webmention."""
    parsed = urlparse(url)
    return parsed.netloc.lower() in ALLOWED_TARGET_DOMAINS


def target_accepts_webmentions(target_url: str) -> bool:
    parsed = urlparse(target_url)
    return parsed.path.startswith(ALLOWED_PATH_PREFIX)


def normalize_url(url: str) -> str:
    """Defrag the URL and strip trailing slashes."""
    defragged, _ = urldefrag(url)
    return defragged.rstrip("/")


def contains_target_link(html_content: str, base_url: str, target_url: str) -> bool:
    """Check if the HTML content contains the target URL"""
    tree = HTMLParser(html_content)
    target_clean = normalize_url(target_url)

    for node in tree.css("a[href]"):
        raw_href = node.attributes.get("href")
        if not raw_href:
            continue

        resolved_href = normalize_url(urljoin(base_url, raw_href))
        if resolved_href == target_clean:
            return True
    return False


async def store_webmention(source: str, target: str) -> None:
    """Store the Webmention in the database."""
    if db_pool is None:
        return
    async with db_pool.acquire() as conn:
        _ = await conn.execute(
            """
            INSERT INTO webmentions (source, target)
            VALUES (?, ?)
            ON CONFLICT (source, target) DO NOTHING
            """,
            (source, target),
        )
        await conn.commit()


async def verify_and_store_webmention(source: str, target: str) -> None:
    """Verify the URL, and store or remove the Webmention accordingly."""
    if not is_allowed_url(source):
        return
    try:
        async with httpx.AsyncClient(
            timeout=FETCH_TIMEOUT,
            follow_redirects=True,
            max_redirects=3,
            headers={"User-Agent": "Webmention-Receiver/1.0 (Verification Bot)"},
        ) as client:
            response = await client.get(source)

            if response.status_code in (410, 404):
                print(f"{source} returned {response.status_code}, removing if present")
                await remove_webmention(source, target)
                return
            elif response.status_code != 200:
                return

            if len(response.content) > MAX_RESPONSE_SIZE:
                return

            html_text = response.text
    except httpx.HTTPError, Exception:
        return

    has_link = contains_target_link(html_text, source, target)
    if has_link:
        print(f"{source} has link to {target}, storing as Webmention!")
        await store_webmention(source, target)
    else:
        print(f"{source} does not have link to {target}, deleting if present!")
        await remove_webmention(source, target)


async def remove_webmention(source: str, target: str) -> None:
    if db_pool is None:
        return
    async with db_pool.acquire() as conn:
        _ = await conn.execute(
            "DELETE FROM webmentions WHERE source = ? AND target = ?", (source, target)
        )
        await conn.commit()


limiter = Limiter(key_func=get_remote_address)
app = FastAPI(lifespan=lifespan)
app.state.limiter = limiter
app.add_exception_handler(RateLimitExceeded, _rate_limit_exceeded_handler)
app.add_middleware(IdleTimeoutMiddleware)

@app.exception_handler(RequestValidationError)
async def validation_exception_handler(
    request: Request, exc: RequestValidationError
) -> JSONResponse:
    errors = exc.errors()

    # If error is URL related, we need
    # to send a 400 Bad Request response
    # as per the Webmention spec.
    is_url_error = any("url" in err.get("type", "") for err in errors)
    if is_url_error:
        return JSONResponse(
            status_code=status.HTTP_400_BAD_REQUEST,
            content={"detail": errors},
        )

    return JSONResponse(
        status_code=status.HTTP_422_UNPROCESSABLE_ENTITY,
        content={"detail": errors},
    )


@app.post("/webmention", status_code=status.HTTP_202_ACCEPTED)
@limiter.limit("10/minute")
async def webmention(
    request: Request,
    source: Annotated[HttpUrl, Form()],
    target: Annotated[HttpUrl, Form()],
    background_tasks: BackgroundTasks,
) -> dict[str, str]:
    source_str = str(source)
    target_str = str(target)
    if source_str == target_str:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Source and target URLs cannot be the same",
        )
    elif not is_allowed_target(target_str):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Target URL {target_str} is not allowed.",
        )
    elif not target_accepts_webmentions(target_str):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Target URL {target_str} does not accept webmentions.",
        )
    background_tasks.add_task(verify_and_store_webmention, source_str, target_str)
    return {"status": "queued"}


@app.get("/get_webmentions", response_class=HTMLResponse)
@limiter.limit("60/minute")
async def get_webmentions_html(
    request: Request, target: Annotated[HttpUrl, Query()]
) -> HTMLResponse:
    if db_pool is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="Database pool uninitialized.",
        )

    target_str = str(target)
    if not is_allowed_target(target_str):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Target URL {target_str} is not allowed.",
        )
    async with db_pool.acquire() as conn:
        rows = await conn.fetchall(
            "SELECT source, created_at FROM webmentions WHERE target = ? ORDER BY created_at ASC;",
            (target_str,),
        )

    if not rows:
        return HTMLResponse('<p class="webmentions_empty">No webmentions yet.</p>')

    rendered_items: list[str] = []
    for row in rows:
        raw_source = row["source"]
        safe_source = html.escape(raw_source)
        # Display the domain or path, escaping for safety
        display_label = html.escape(urlparse(raw_source).netloc or raw_source)
        date_str = html.escape(str(row["created_at"]).split()[0])

        rendered_items.append(
            f"""
            <li class="webmention">
                <a href="{safe_source}" rel="nofollow noopener noreferrer" class="webmention_link">{display_label}</a>
                <time class="webmention_date" datetime="{date_str}">{date_str}</time>
            </li>
            """
        )

    return HTMLResponse(
        f"""
        <ul class="webmentions_container">
            {"\n".join(rendered_items)}
        </ul>
        """
    )
