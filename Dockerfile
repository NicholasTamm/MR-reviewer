FROM python:3.11-slim

WORKDIR /app

COPY pyproject.toml .
COPY mr_reviewer/ ./mr_reviewer/

RUN pip install --no-cache-dir -e ".[all-providers]"

ENTRYPOINT ["python", "-m", "mr_reviewer"]
CMD []
