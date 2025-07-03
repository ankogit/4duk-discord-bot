#!/bin/bash

echo "🚀 Запуск Discord Radio Bot"

# Проверяем наличие .env файла
if [ ! -f .env ]; then
    echo "❌ Файл .env не найден!"
    echo "Создайте файл .env с DISCORD_TOKEN=your_token_here"
    exit 1
fi

# Функция для проверки интернет соединения
check_internet() {
    echo "🌐 Проверяю интернет соединение..."
    if ping -c 1 8.8.8.8 > /dev/null 2>&1; then
        echo "✅ Интернет доступен"
        return 0
    else
        echo "❌ Нет интернет соединения"
        return 1
    fi
}

# Функция для запуска диагностики
run_diagnostics() {
    echo "🔍 Запускаю сетевую диагностику..."
    python3 diagnose.py
}

# Функция для остановки контейнера
stop_bot() {
    echo "⏹️  Останавливаю бота..."
    docker-compose down
}

# Функция для запуска бота
start_bot() {
    echo "🤖 Запускаю Discord бота..."
    docker-compose up --build -d
    
    echo "📋 Статус контейнера:"
    docker-compose ps
    
    echo "📄 Последние логи:"
    docker-compose logs --tail=20 radio
}

# Функция для просмотра логов
show_logs() {
    echo "📄 Логи бота:"
    docker-compose logs -f radio
}

# Обработка аргументов командной строки
case "$1" in
    "stop")
        stop_bot
        ;;
    "logs")
        show_logs
        ;;
    "restart")
        stop_bot
        sleep 2
        start_bot
        ;;
    "diagnose")
        run_diagnostics
        ;;
    "status")
        docker-compose ps
        ;;
    *)
        check_internet
        if [ $? -eq 0 ]; then
            start_bot
        else
            echo "❌ Запуск невозможен из-за проблем с сетью"
            exit 1
        fi
        ;;
esac

echo "✨ Готово!"
echo ""
echo "Доступные команды:"
echo "  ./start.sh         - запустить бота"
echo "  ./start.sh stop    - остановить бота"  
echo "  ./start.sh restart - перезапустить бота"
echo "  ./start.sh logs    - показать логи"
echo "  ./start.sh diagnose- запустить диагностику"
echo "  ./start.sh status  - показать статус"