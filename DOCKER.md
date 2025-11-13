# Запуск в Docker

## Быстрый старт

1. Убедитесь, что Docker Desktop запущен
2. Создайте файл `.env` с токеном:
   ```
   DISCORD_TOKEN=your_token_here
   RADIO_URL=http://radio.4duk.ru/4duk128.mp3
   ```

3. Запустите одним из способов:

### Способ 1: Docker Compose (рекомендуется)

```bash
# Сборка и запуск с логами
docker compose up --build

# Или в фоновом режиме
docker compose up -d --build

# Просмотр логов
docker compose logs -f

# Остановка
docker compose down
```

### Способ 2: Скрипт

```bash
./run-docker.sh
```

### Способ 3: Docker напрямую

```bash
# Сборка образа
docker build -t 4duk-discord-bot .

# Запуск с логами
docker run --rm -it \
    --name discord-radio-bot \
    --env-file .env \
    4duk-discord-bot

# Или в фоновом режиме
docker run -d \
    --name discord-radio-bot \
    --env-file .env \
    --restart unless-stopped \
    4duk-discord-bot

# Просмотр логов
docker logs -f discord-radio-bot
```

## Проверка работы

После запуска вы должны увидеть логи:
```
INFO[2025-11-13T...] Bot started successfully
INFO[2025-11-13T...] Bot ready as <bot_name> (ID: ...)
```

Если видите ошибки, проверьте:
- Запущен ли Docker Desktop
- Существует ли файл `.env` с правильным токеном
- Доступен ли интернет для подключения к Discord

