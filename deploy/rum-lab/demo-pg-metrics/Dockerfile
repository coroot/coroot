FROM python:3.12-alpine
WORKDIR /app
RUN apk add --no-cache snappy-dev gcc musl-dev \
 && pip install --no-cache-dir python-snappy \
 && apk del gcc musl-dev
COPY remote_write.py app.py /app/
ENV PYTHONUNBUFFERED=1
CMD ["python", "/app/app.py"]
