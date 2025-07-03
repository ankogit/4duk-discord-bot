#!/usr/bin/env python3
"""
Скрипт для диагностики сетевых проблем с Discord API
"""

import asyncio
import aiohttp
import socket
import subprocess
import sys

DISCORD_ENDPOINTS = [
    "discord.com",
    "gateway.discord.gg",
    "gateway-us-east1-c.discord.gg",
    "api.discord.com"
]

DNS_SERVERS = [
    "8.8.8.8",
    "8.8.4.4", 
    "1.1.1.1",
    "208.67.222.222"
]

async def test_dns_resolution():
    """Тестирует DNS разрешение для Discord серверов"""
    print("=== Тестирование DNS разрешения ===")
    
    for endpoint in DISCORD_ENDPOINTS:
        try:
            addr_info = socket.getaddrinfo(endpoint, 443)
            ip = addr_info[0][4][0]
            print(f"✅ {endpoint} -> {ip}")
        except socket.gaierror as e:
            print(f"❌ {endpoint} -> Ошибка: {e}")
        except Exception as e:
            print(f"❌ {endpoint} -> Неожиданная ошибка: {e}")

def test_ping():
    """Тестирует ping до DNS серверов"""
    print("\n=== Тестирование ping до DNS серверов ===")
    
    for dns in DNS_SERVERS:
        try:
            result = subprocess.run(
                ["ping", "-c", "3", dns], 
                capture_output=True, 
                text=True, 
                timeout=10
            )
            if result.returncode == 0:
                print(f"✅ {dns} - доступен")
            else:
                print(f"❌ {dns} - недоступен")
        except subprocess.TimeoutExpired:
            print(f"❌ {dns} - таймаут")
        except Exception as e:
            print(f"❌ {dns} - ошибка: {e}")

async def test_http_connection():
    """Тестирует HTTP подключение к Discord API"""
    print("\n=== Тестирование HTTP подключения ===")
    
    # Настройка коннектора с кастомными DNS
    connector = aiohttp.TCPConnector(
        resolver=aiohttp.resolver.AsyncResolver(nameservers=DNS_SERVERS),
        ttl_dns_cache=300,
        use_dns_cache=True
    )
    
    timeout = aiohttp.ClientTimeout(total=30)
    
    async with aiohttp.ClientSession(connector=connector, timeout=timeout) as session:
        for endpoint in DISCORD_ENDPOINTS:
            try:
                url = f"https://{endpoint}"
                async with session.get(url) as response:
                    print(f"✅ {url} -> HTTP {response.status}")
            except aiohttp.ClientConnectorDNSError as e:
                print(f"❌ {url} -> DNS ошибка: {e}")
            except aiohttp.ClientConnectorError as e:
                print(f"❌ {url} -> Ошибка подключения: {e}")
            except asyncio.TimeoutError:
                print(f"❌ {url} -> Таймаут")
            except Exception as e:
                print(f"❌ {url} -> Ошибка: {e}")

def check_system_dns():
    """Проверяет системные настройки DNS"""
    print("\n=== Системные настройки DNS ===")
    
    try:
        with open("/etc/resolv.conf", "r") as f:
            content = f.read()
            print("Содержимое /etc/resolv.conf:")
            print(content)
    except Exception as e:
        print(f"Ошибка чтения /etc/resolv.conf: {e}")

async def main():
    print("🔍 Запускаю диагностику сетевых проблем...\n")
    
    check_system_dns()
    await test_dns_resolution()
    test_ping()
    await test_http_connection()
    
    print("\n📋 Рекомендации:")
    print("1. Убедитесь что интернет соединение стабильно")
    print("2. Проверьте настройки DNS в docker-compose.yml")
    print("3. Попробуйте перезапустить контейнер: docker-compose down && docker-compose up")
    print("4. При повторных проблемах используйте network_mode: host")

if __name__ == "__main__":
    asyncio.run(main())