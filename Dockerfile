FROM python:3.11-slim

# Установка зависимостей системы и сетевых утилит
RUN apt-get update && apt-get install -y \
    build-essential \
    libffi-dev \
    python3-dev \
    ffmpeg \
    dnsutils \
    iputils-ping \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Настройка DNS в контейнере
RUN echo 'nameserver 8.8.8.8' > /etc/resolv.conf && \
    echo 'nameserver 8.8.4.4' >> /etc/resolv.conf && \
    echo 'nameserver 1.1.1.1' >> /etc/resolv.conf

WORKDIR /app

COPY requirements.txt .

RUN pip install --no-cache-dir --upgrade pip && \
    pip install --no-cache-dir -r requirements.txt

COPY . .

# Добавляем health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

CMD ["python", "bot.py"]
