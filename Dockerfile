FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    DATABASE=/var/lib/material-library/data/app.db \
    UPLOAD_ROOT=/var/lib/material-library/uploads

WORKDIR /app
COPY requirements.txt .
RUN python -m pip install --no-cache-dir -r requirements.txt \
    && groupadd --gid 10001 app \
    && useradd --uid 10001 --gid app --no-create-home app \
    && mkdir -p /var/lib/material-library/data /var/lib/material-library/uploads/tmp /var/lib/material-library/uploads/files \
    && chown -R app:app /var/lib/material-library

COPY app ./app
COPY scripts ./scripts
COPY wsgi.py ./wsgi.py

USER 10001:10001
EXPOSE 8080
CMD ["python", "scripts/serve.py"]
