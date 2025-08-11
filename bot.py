import os
import asyncio
import discord
from discord.ext import commands, tasks
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
reconnect_attempts = {}

async def start_radio(vc: discord.VoiceClient, guild_id: int):
    """Запускает радио и перезапускает при обрыве"""
    def after_play(error):
        if error:
            print(f"[ERROR] Поток радио завершился с ошибкой: {error}")
        if hasattr(vc, "source") and vc.source:
            try:
                vc.source.cleanup()
            except Exception as e:
                print(f"[WARN] Ошибка при cleanup источника: {e}")
        if radio_status.get(guild_id, False):
            asyncio.run_coroutine_threadsafe(reconnect_radio(vc, guild_id), bot.loop)

    if vc.is_playing():
        vc.stop()
    if hasattr(vc, "source") and vc.source:
        try:
            vc.source.cleanup()
        except:
            pass

    print(f"[INFO] Запуск радио для {guild_id}")
    vc.play(FFmpegPCMAudio(RADIO_URL, **FFMPEG_OPTIONS), after=after_play)

async def reconnect_radio(vc: discord.VoiceClient, guild_id: int):
    """Переподключает радио после обрыва"""
    await asyncio.sleep(2)
    reconnect_attempts[guild_id] = reconnect_attempts.get(guild_id, 0) + 1

    try:
        if not vc.is_connected():
            channel = vc.channel
            await vc.disconnect(force=True)
            new_vc = await channel.connect(reconnect=False)
            reconnect_attempts[guild_id] = 0
            await start_radio(new_vc, guild_id)
            return
        await start_radio(vc, guild_id)

    except discord.errors.ConnectionClosed as e:
        print(f"[ERROR] Voice WS закрыт: {e}")
        if e.code == 4006 or reconnect_attempts[guild_id] > 3:
            print("[WARN] Сессия недействительна, пересоздаю подключение...")
            try:
                channel = vc.channel
                await vc.disconnect(force=True)
                new_vc = await channel.connect(reconnect=False)
                reconnect_attempts[guild_id] = 0
                await start_radio(new_vc, guild_id)
            except Exception as ex:
                print(f"[FATAL] Не удалось пересоздать подключение: {ex}")

    except Exception as e:
        print(f"[ERROR] Ошибка при переподключении: {e}")

@bot.event
async def on_ready():
    print(f"Бот запущен как {bot.user}")
    voice_check_loop.start()

@bot.command()
@bot.command()
async def join(ctx):
    """Подключает бота к голосовому каналу"""
    if not ctx.author.voice:
        await ctx.send("Ты не в голосовом канале!")
        return

    channel = ctx.author.voice.channel
    try:
        vc = await channel.connect(reconnect=False)
        await ctx.send(f"Подключился к {channel.name}")
    except discord.errors.ConnectionClosed as e:
        if e.code == 4006:
            await ctx.send("Сессия устарела, пробую переподключиться...")
            try:
                # Делаем полный реконнект
                if ctx.voice_client:
                    await ctx.voice_client.disconnect(force=True)
                vc = await channel.connect(reconnect=False)
                await ctx.send(f"Подключился к {channel.name} после реконнекта")
            except Exception as ex:
                await ctx.send(f"❌ Не удалось подключиться: {ex}")
        else:
            await ctx.send(f"❌ Ошибка подключения: {e}")
    except Exception as e:
        await ctx.send(f"❌ Не удалось подключиться: {e}")

@bot.command()
async def radio(ctx):
    if not ctx.voice_client:
        if ctx.author.voice:
            vc = await ctx.author.voice.channel.connect(reconnect=False)
        else:
            await ctx.send("Ты не в голосовом канале!")
            return
    else:
        vc = ctx.voice_client

    radio_status[ctx.guild.id] = True
    reconnect_attempts[ctx.guild.id] = 0
    await start_radio(vc, ctx.guild.id)
    await ctx.send("🎵 Вещаю радио!")

@bot.command()
async def stop(ctx):
    radio_status[ctx.guild.id] = False
    if ctx.voice_client:
        await ctx.voice_client.disconnect()
        await ctx.send("Отключился.")
    else:
        await ctx.send("Я не в голосовом канале.")

@tasks.loop(seconds=30)
async def voice_check_loop():
    for guild_id, active in list(radio_status.items()):
        if not active:
            continue
        guild = bot.get_guild(guild_id)
        if not guild or not guild.voice_client:
            continue
        vc = guild.voice_client
        if not vc.is_connected() or not vc.is_playing():
            print(f"[INFO] Автовосстановление радио для {guild_id}")
            await reconnect_radio(vc, guild_id)

bot.run(os.getenv("DISCORD_TOKEN"))
