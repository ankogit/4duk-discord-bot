# bot.py
import os
import asyncio
import logging
import signal
from typing import Optional, Dict

import discord
from discord.ext import commands, tasks
from discord import FFmpegPCMAudio
from dotenv import load_dotenv

load_dotenv()

# Logging
logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("radio-bot")

# Intents
intents = discord.Intents.default()
intents.message_content = True
intents.voice_states = True
intents.guilds = True

bot = commands.Bot(command_prefix="!", intents=intents)

# Radio settings
RADIO_URL = os.getenv("RADIO_URL", "http://radio.4duk.ru/4duk128.mp3")
FFMPEG_OPTIONS = {
    "before_options": "-reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 5 -reconnect_at_eof 1",
    "options": "-vn",
}

# State per guild:
# radio_state[guild_id] = {
#     "active": bool,
#     "channel_id": int or None,
#     "reconnect_attempts": int,
#     "lock": asyncio.Lock()
# }
radio_state: Dict[int, Dict] = {}

# Config
MAX_RECONNECT_ATTEMPTS = 5
RECONNECT_BACKOFF_BASE = 2  # seconds
VOICE_CHECK_INTERVAL = 20  # seconds


def ensure_guild_state(guild_id: int):
    if guild_id not in radio_state:
        radio_state[guild_id] = {
            "active": False,
            "channel_id": None,
            "reconnect_attempts": 0,
            "lock": asyncio.Lock(),
        }


async def safe_cleanup_source(vc: discord.VoiceClient):
    """Try to clean up old ffmpeg/Source safely."""
    try:
        if vc.is_playing():
            vc.stop()
    except Exception:
        pass

    # cleanup() exists on some Source implementations
    try:
        src = getattr(vc, "source", None)
        if src and hasattr(src, "cleanup"):
            try:
                src.cleanup()
            except Exception:
                pass
    except Exception:
        pass


async def start_radio_for_vc(vc: discord.VoiceClient, guild_id: int):
    """Start playing radio on given VoiceClient, with after callback to attempt reconnects."""
    def after_play(error):
        if error:
            log.warning(f"[{guild_id}] after_play error: {error}")
        # cleanup old source and schedule reconnect if still active
        try:
            # run coroutine in bot loop
            asyncio.run_coroutine_threadsafe(safe_cleanup_source(vc), bot.loop)
        except Exception as e:
            log.debug(f"[{guild_id}] after_play cleanup scheduling failed: {e}")

        # schedule reconnect
        state = radio_state.get(guild_id)
        if state and state["active"]:
            # schedule reconnect task
            try:
                asyncio.run_coroutine_threadsafe(reconnect_radio(guild_id), bot.loop)
            except Exception as e:
                log.error(f"[{guild_id}] Failed to schedule reconnect: {e}")

    try:
        await safe_cleanup_source(vc)
        log.info(f"[{guild_id}] Playing radio: {RADIO_URL}")
        source = FFmpegPCMAudio(RADIO_URL, **FFMPEG_OPTIONS)
        vc.play(source, after=after_play)
    except Exception as e:
        log.exception(f"[{guild_id}] Failed to start radio: {e}")
        # schedule reconnect
        asyncio.create_task(reconnect_radio(guild_id))


async def connect_to_channel(guild: discord.Guild, channel: discord.VoiceChannel) -> Optional[discord.VoiceClient]:
    """Try to connect to voice channel with safe error handling."""
    try:
        # If bot already has a voice client in that guild, move it
        existing = discord.utils.get(bot.voice_clients, guild=guild)
        if existing and existing.is_connected():
            try:
                await existing.move_to(channel)
                return existing
            except Exception as e:
                log.warning(f"[{guild.id}] move_to failed: {e}")
                # try disconnect and reconnect
                try:
                    await existing.disconnect(force=True)
                except Exception:
                    pass

        # Try connecting (allow library to manage resume attempts)
        vc = await channel.connect(reconnect=True)
        return vc
    except discord.errors.ConnectionClosed as e:
        log.warning(f"[{guild.id}] ConnectionClosed during connect: {getattr(e, 'code', None)} - {e}")
        # If session invalid (4006) or any ConnectionClosed, do force reconnect attempt
        try:
            existing = discord.utils.get(bot.voice_clients, guild=guild)
            if existing:
                try:
                    await existing.disconnect(force=True)
                except Exception:
                    pass
            # small sleep to give Discord time
            await asyncio.sleep(1)
            vc = await channel.connect(reconnect=True)
            return vc
        except Exception as ex:
            log.error(f"[{guild.id}] Force reconnect failed: {ex}")
            return None
    except Exception as e:
        log.exception(f"[{guild.id}] Unexpected error while connecting: {e}")
        return None


async def reconnect_radio(guild_id: int):
    """Attempt to reconnect the radio for the guild. Handles exponential backoff and session-invalid cases."""
    ensure_guild_state(guild_id)
    state = radio_state[guild_id]
    async with state["lock"]:
        if not state["active"]:
            log.info(f"[{guild_id}] Radio not active anymore, skipping reconnect.")
            return

        guild = bot.get_guild(guild_id)
        if not guild:
            log.warning(f"[{guild_id}] Guild object not found.")
            return

        channel_id = state.get("channel_id")
        if not channel_id:
            log.warning(f"[{guild_id}] No channel recorded to reconnect.")
            return

        channel = guild.get_channel(channel_id)
        if not channel or not isinstance(channel, discord.VoiceChannel):
            log.warning(f"[{guild_id}] Channel {channel_id} not found or not a voice channel.")
            return

        # If we already have a voice client and it's connected and playing, nothing to do
        vc = discord.utils.get(bot.voice_clients, guild=guild)
        if vc and vc.is_connected() and vc.is_playing():
            log.info(f"[{guild_id}] VC already connected and playing.")
            return

        # Backoff if too many attempts
        attempts = state.get("reconnect_attempts", 0)
        if attempts >= MAX_RECONNECT_ATTEMPTS:
            log.error(f"[{guild_id}] Reached max reconnect attempts ({attempts}). Giving up until user restarts radio.")
            return

        # Sleep with backoff
        backoff = RECONNECT_BACKOFF_BASE ** attempts
        log.info(f"[{guild_id}] Reconnect attempt #{attempts + 1}, sleeping {backoff}s before trying.")
        await asyncio.sleep(backoff)

        state["reconnect_attempts"] = attempts + 1

        try:
            vc = await connect_to_channel(guild, channel)
            if not vc:
                log.error(f"[{guild_id}] connect_to_channel returned None.")
                # schedule another attempt (if still active)
                if state["active"] and state["reconnect_attempts"] < MAX_RECONNECT_ATTEMPTS:
                    asyncio.create_task(reconnect_radio(guild_id))
                return

            # reset attempts on success
            state["reconnect_attempts"] = 0
            await start_radio_for_vc(vc, guild_id)
        except Exception as e:
            log.exception(f"[{guild_id}] Exception in reconnect_radio: {e}")
            # schedule retry
            if state["active"] and state["reconnect_attempts"] < MAX_RECONNECT_ATTEMPTS:
                asyncio.create_task(reconnect_radio(guild_id))


@bot.event
async def on_ready():
    log.info(f"Bot ready as {bot.user} (id={bot.user.id})")
    if not voice_check_loop.is_running():
        voice_check_loop.start()


@bot.command(name="join")
async def cmd_join(ctx: commands.Context):
    """Подключает бота к голосовому каналу автора команды."""
    if not ctx.author.voice or not ctx.author.voice.channel:
        await ctx.send("Ты не в голосовом канале!")
        return

    channel = ctx.author.voice.channel
    guild = ctx.guild
    ensure_guild_state(guild.id)

    try:
        vc = await connect_to_channel(guild, channel)
        if vc:
            # store channel for auto-reconnects
            radio_state[guild.id]["channel_id"] = channel.id
            await ctx.send(f"Подключился к {channel.name}")
        else:
            await ctx.send("Не удалось подключиться к голосовому каналу. Смотри логи.")
    except Exception as e:
        log.exception(f"[{guild.id}] join command failed: {e}")
        await ctx.send(f"Ошибка при подключении: {e}")


@bot.command(name="radio")
async def cmd_radio(ctx: commands.Context):
    """Включает радио в голосовом канале автора."""
    if not ctx.author.voice or not ctx.author.voice.channel:
        await ctx.send("Ты не в голосовом канале!")
        return

    channel = ctx.author.voice.channel
    guild = ctx.guild
    ensure_guild_state(guild.id)

    # mark active and store channel
    radio_state[guild.id]["active"] = True
    radio_state[guild.id]["channel_id"] = channel.id
    radio_state[guild.id]["reconnect_attempts"] = 0

    try:
        vc = await connect_to_channel(guild, channel)
        if not vc:
            await ctx.send("Не удалось подключиться к голосовому каналу для радио.")
            return

        await start_radio_for_vc(vc, guild.id)
        await ctx.send("🎵 Вещаю радио!")
    except Exception as e:
        log.exception(f"[{guild.id}] radio command failed: {e}")
        await ctx.send(f"Ошибка при запуске радио: {e}")


@bot.command(name="stop")
async def cmd_stop(ctx: commands.Context):
    """Останавливает радио и отключает бота."""
    guild = ctx.guild
    ensure_guild_state(guild.id)

    radio_state[guild.id]["active"] = False
    radio_state[guild.id]["reconnect_attempts"] = 0

    vc = discord.utils.get(bot.voice_clients, guild=guild)
    try:
        if vc and vc.is_connected():
            await safe_cleanup_source(vc)
            await vc.disconnect(force=True)
            await ctx.send("Отключился.")
        else:
            await ctx.send("Я не в голосовом канале.")
    except Exception as e:
        log.exception(f"[{guild.id}] stop command failed: {e}")
        await ctx.send(f"Ошибка при отключении: {e}")


@tasks.loop(seconds=VOICE_CHECK_INTERVAL)
async def voice_check_loop():
    """Периодическая проверка — если радио помечено активным, но нет воспроизведения — триггерим reconnect."""
    for guild_id, state in list(radio_state.items()):
        if not state.get("active"):
            continue
        guild = bot.get_guild(guild_id)
        if not guild:
            continue
        vc = discord.utils.get(bot.voice_clients, guild=guild)
        if not vc or not vc.is_connected() or not vc.is_playing():
            log.info(f"[{guild_id}] voice_check_loop: detected dead vc -> scheduling reconnect.")
            asyncio.create_task(reconnect_radio(guild_id))


# Clean shutdown on SIGTERM/SIGINT to avoid dangling ffmpeg processes in container
def _shutdown():
    log.info("Shutting down...")
    for vc in list(bot.voice_clients):
        try:
            asyncio.create_task(safe_cleanup_source(vc))
            asyncio.create_task(vc.disconnect(force=True))
        except Exception:
            pass
    try:
        loop = asyncio.get_event_loop()
        loop.stop()
    except Exception:
        pass


if __name__ == "__main__":
    # Install signal handlers when running in Docker / systemd
    try:
        loop = asyncio.get_event_loop()
        for sig in (signal.SIGINT, signal.SIGTERM):
            loop.add_signal_handler(sig, _shutdown)
    except Exception:
        pass

    TOKEN = os.getenv("DISCORD_TOKEN")
    if not TOKEN:
        log.error("DISCORD_TOKEN not set in environment")
        raise SystemExit(1)

    bot.run(TOKEN)
