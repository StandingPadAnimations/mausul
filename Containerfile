# Building the backend. We do this in a
# separate stage since we need uv at this
# point. Also, disable dev dependencies and
# Python downlods for uv, since we want to
# keep final image sizes down.
FROM ghcr.io/astral-sh/uv:python3.14-trixie-slim AS backend-builder
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy UV_NO_DEV=1 UV_PYTHON_DOWNLOADS=0

# Mount everything so we don't need copy
# steps. Also, we don't have a separate
# build stage for the backend, so we can
# get away with one uv sync command.
WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/uv \
    --mount=type=bind,source=uv.lock,target=/app/uv.lock \
    --mount=type=bind,source=pyproject.toml,target=/app/pyproject.toml \
    uv sync --locked --no-install-project

# Final production image
FROM python:3.14-slim
WORKDIR /app

# This pretty much should always be cached
# anyways, no point in splitting these commands
# into their own layers.
RUN groupadd --system --gid 999 nonroot \
 && useradd --system --gid 999 --uid 999 --create-home nonroot \
 && mkdir -p /webmentions && chown -R nonroot:nonroot /webmentions

# Only copy the virtual environment to reduce
# final image size. Also use PYTHONUNBUFFERED
# to make sure we have logs in case the app
# crashes.
COPY --from=backend-builder --chown=nonroot:nonroot /app/.venv /app/.venv
ENV PATH="/app/.venv/bin:$PATH" PYTHONUNBUFFERED=1

# Do these separately for caching purposes. We
# could *possibly* also split the backend and
# common copies, but like with the frontend, they're
# so small that it's not really worth the additional
# layer for caching. The backend and frontend as a
# whole are cached individually though.
COPY --chown=nonroot:nonroot ./src/webmentions/ /app/webmentions

USER nonroot
EXPOSE 8000
CMD ["uvicorn", "webmentions:app", "--fd", "3"]
