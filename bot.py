import os
import discord
from discord.ext import commands
from discord import FFmpegPCMAudio
from dotenv import load_dotenv

load_dotenv()

intents = discord.Intents.default()
intents.message_content = True  # Нужен для команд по сообщениям
intents.voice_states = True     # Нужен для работы с голосовыми каналами
intents.guilds = True           # Стандартный доступ к серверам

bot = commands.Bot(command_prefix="!", intents=intents)

RADIO_URL = "http://radio.4duk.ru/4duk128.mp3"
FFMPEG_OPTIONS = {
    'before_options': '-reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 5',
    'options': '-vn'
}

@bot.event
async def on_ready():
    print(f"Бот запущен как {bot.user}")

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
    
bot.run(os.getenv("DISCORD_TOKEN"))
