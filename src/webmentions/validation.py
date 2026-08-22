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

import ipaddress
import socket
from urllib.parse import urldefrag, urljoin, urlparse

from selectolax.parser import HTMLParser

ALLOWED_TARGET_DOMAINS = {
    "standingpad.org",
    "www.standingpad.org",
    "localhost",
    "127.0.0.1",
}
ALLOWED_PATH_PREFIX = "/posts/"


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
