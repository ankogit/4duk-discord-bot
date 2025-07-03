import os
import discord
from discord.ext import commands
from discord import FFmpegPCMAudio
from dotenv import load_dotenv
import asyncio
import aiohttp
import logging
import signal
import sys

# Настройка логирования
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)

load_dotenv()

intents = discord.Intents.default()
intents.message_content = True  # Нужен для команд по сообщениям
intents.voice_states = True     # Нужен для работы с голосовыми каналами
intents.guilds = True           # Стандартный доступ к серверам

# Настройка кастомного коннектора с лучшими DNS настройками
connector = aiohttp.TCPConnector(
    limit=100,
    limit_per_host=30,
    ttl_dns_cache=300,
    use_dns_cache=True,
    resolver=aiohttp.resolver.AsyncResolver(nameservers=['8.8.8.8', '8.8.4.4', '1.1.1.1'])
)

bot = commands.Bot(
    command_prefix="!",
    intents=intents,
    connector=connector,
    timeout=60.0,
    max_messages=10000
)

RADIO_URL = "http://radio.4duk.ru/4duk128.mp3"
FFMPEG_OPTIONS = {
    'before_options': '-reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 5',
    'options': '-vn'
}

async def run_bot_with_retry():
    """Запуск бота с повторными попытками при ошибках подключения"""
    retry_count = 0
    max_retries = 5
    
    while retry_count < max_retries:
        try:
            await bot.start(os.getenv("DISCORD_TOKEN"))
        except (aiohttp.ClientConnectorDNSError, aiohttp.ClientConnectorError, discord.ConnectionClosed) as e:
            retry_count += 1
            wait_time = min(60 * retry_count, 300)  # Максимум 5 минут
            logging.error(f"Ошибка подключения (попытка {retry_count}/{max_retries}): {e}")
            logging.info(f"Повторная попытка через {wait_time} секунд...")
            await asyncio.sleep(wait_time)
        except Exception as e:
            logging.error(f"Неожиданная ошибка: {e}")
            break
    
    logging.error("Превышено максимальное количество попыток подключения")

@bot.event
async def on_ready():
    logging.info(f"Бот запущен как {bot.user}")
    print(f"Бот запущен как {bot.user}")

@bot.event
async def on_disconnect():
    logging.warning("Бот отключен от Discord")

@bot.event
async def on_resumed():
    logging.info("Соединение с Discord восстановлено")

@bot.command()
async def join(ctx):
    if ctx.author.voice:
        channel = ctx.author.voice.channel
        await channel.connect()
        await ctx.send(f"Подключился к {channel.name}")
    else:
        await ctx.send("Ты не в голосовом канале!")

@bot.command()
async def radio(ctx):
    vc = ctx.voice_client
    if not vc:
        if ctx.author.voice:
            vc = await ctx.author.voice.channel.connect()
        else:
            await ctx.send("Ты не в голосовом канале!")
            return
    vc.play(FFmpegPCMAudio(RADIO_URL, **FFMPEG_OPTIONS))
    await ctx.send("🎵 Вещаю радио!")

@bot.command()
async def stop(ctx):
    if ctx.voice_client:
        await ctx.voice_client.disconnect()
        await ctx.send("Отключился.")
    else:
        await ctx.send("Я не в голосовом канале.")

@bot.command()
async def ping(ctx):
    await ctx.send("Pong!")

# Graceful shutdown
def signal_handler(signum, frame):
    logging.info("Получен сигнал завершения, закрываю бота...")
    asyncio.create_task(bot.close())
    sys.exit(0)

signal.signal(signal.SIGINT, signal_handler)
signal.signal(signal.SIGTERM, signal_handler)

if __name__ == "__main__":
    try:
        asyncio.run(run_bot_with_retry())
    except KeyboardInterrupt:
        logging.info("Бот остановлен пользователем")
    finally:
        asyncio.run(bot.close())
