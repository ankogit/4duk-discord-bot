#!/bin/bash

# Скрипт для запуска бота в Docker с выводом логов

echo "Building Docker image..."
docker build -t 4duk-discord-bot .

if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo ""
echo "Starting container with logs..."
echo "Press Ctrl+C to stop"
echo ""

# Запускаем контейнер с выводом логов и автоматическим удалением при остановке
docker run --rm -it \
    --name discord-radio-bot \
    --env-file .env \
    4duk-discord-bot

