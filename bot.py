import os
import asyncio
import discord
from discord.ext import commands
from discord import FFmpegPCMAudio
from dotenv import load_dotenv

load_dotenv()

intents = discord.Intents.default()
intents.message_content = True
intents.voice_states = True
intents.guilds = True

bot = commands.Bot(command_prefix="!", intents=intents)

RADIO_URL = "http://radio.4duk.ru/4duk128.mp3"
FFMPEG_OPTIONS = {
    'before_options': '-reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 5 -reconnect_at_eof 1',
    'options': '-vn'
}

# Храним статус радио для каждого сервера
radio_status = {}

async def start_radio(vc: discord.VoiceClient, guild_id: int):
    """Запускает радио и перезапускает при обрыве"""
    def after_play(error):
        if error:
            print(f"[ERROR] Поток радио завершился с ошибкой: {error}")
        # Если радио всё ещё активно в этом guild
        if radio_status.get(guild_id, False):
            asyncio.run_coroutine_threadsafe(reconnect_radio(vc, guild_id), bot.loop)

    if vc.is_playing():
        vc.stop()

    print(f"[INFO] Запуск радио для {guild_id}")
    vc.play(FFmpegPCMAudio(RADIO_URL, **FFMPEG_OPTIONS), after=after_play)

async def reconnect_radio(vc: discord.VoiceClient, guild_id: int):
    """Переподключает радио после обрыва"""
    await asyncio.sleep(2)  # небольшая задержка перед перезапуском
    if vc.is_connected():
        await start_radio(vc, guild_id)

@bot.event
async def on_ready():
    print(f"Бот запущен как {bot.user}")

@bot.command()
async def join(ctx):
    """Подключает бота к голосовому каналу"""
    if ctx.author.voice:
        channel = ctx.author.voice.channel
        await channel.connect()
        await ctx.send(f"Подключился к {channel.name}")
    else:
        await ctx.send("Ты не в голосовом канале!")

@bot.command()
async def radio(ctx):
    """Включает радио"""
    if not ctx.voice_client:
        if ctx.author.voice:
            vc = await ctx.author.voice.channel.connect()
        else:
            await ctx.send("Ты не в голосовом канале!")
            return
    else:
        vc = ctx.voice_client

    radio_status[ctx.guild.id] = True
    await start_radio(vc, ctx.guild.id)
    await ctx.send("🎵 Вещаю радио!")

@bot.command()
async def stop(ctx):
    """Останавливает радио и отключается"""
    radio_status[ctx.guild.id] = False
    if ctx.voice_client:
        await ctx.voice_client.disconnect()
        await ctx.send("Отключился.")
    else:
        await ctx.send("Я не в голосовом канале.")

@bot.command()
async def ping(ctx):
    await ctx.send("Pong!")

# Автопереподключение при разрыве voice WebSocket
@bot.event
async def on_disconnect():
    print("[WARN] Потеряно подключение к Discord WebSocket — попытка восстановления...")

@bot.event
async def on_resumed():
    print("[INFO] Сессия возобновлена.")

bot.run(os.getenv("DISCORD_TOKEN"))
