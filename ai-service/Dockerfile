ARG BASE_IMAGE_REGISTRY=
FROM ${BASE_IMAGE_REGISTRY}python:3.12-slim@sha256:dddfd7e07f9d15aeeca61529320492139d21cac7f0070c00609243e51e4e0016

ENV PYTHONUNBUFFERED=1
ENV DEBIAN_FRONTEND=noninteractive
WORKDIR /app

ARG DEBIAN_MIRROR=https://deb.debian.org
RUN sed -i "s|http://deb.debian.org|${DEBIAN_MIRROR}|g; s|https://deb.debian.org|${DEBIAN_MIRROR}|g" /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
        -o Acquire::Retries=8 \
        -o Acquire::http::Timeout=30 \
        -o Acquire::https::Timeout=30 \
        -o Acquire::http::No-Cache=true \
        -o Acquire::http::Pipeline-Depth=0 \
    && apt-get install -y --no-install-recommends \
        libreoffice-writer \
        fonts-wqy-zenhei \
    && rm -rf /var/lib/apt/lists/*

COPY pyproject.toml ./
COPY requirements.lock ./
COPY app ./app
ARG INSTALL_DEV_DEPS=true
ARG PIP_INDEX_URL=https://pypi.org/simple
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --index-url "$PIP_INDEX_URL" --only-binary=:all: --no-deps -r requirements.lock

EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
