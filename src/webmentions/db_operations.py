# Copyright (C) 2026 Maryam Stellamaris (Mahid Sheikh)
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

import asqlite


async def init_db(db_pool: asqlite.Pool) -> None:
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


async def store_webmention(db_pool: asqlite.Pool, source: str, target: str) -> None:
    """Store the Webmention in the database."""
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


async def remove_webmention(db_pool: asqlite.Pool, source: str, target: str) -> None:
    """Remove the Webmention from the database."""
    async with db_pool.acquire() as conn:
        _ = await conn.execute(
            "DELETE FROM webmentions WHERE source = ? AND target = ?", (source, target)
        )
        await conn.commit()
