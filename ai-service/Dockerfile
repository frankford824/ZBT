ARG BASE_IMAGE_REGISTRY=
FROM ${BASE_IMAGE_REGISTRY}python:3.12-slim

ENV PYTHONUNBUFFERED=1
ENV DEBIAN_FRONTEND=noninteractive
WORKDIR /app

RUN sed -i 's|http://deb.debian.org|https://deb.debian.org|g' /etc/apt/sources.list.d/debian.sources \
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
COPY app ./app
ARG INSTALL_DEV_DEPS=true
RUN if [ "$INSTALL_DEV_DEPS" = "true" ]; then \
        pip install --no-cache-dir ".[dev]"; \
    else \
        pip install --no-cache-dir .; \
    fi

EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
