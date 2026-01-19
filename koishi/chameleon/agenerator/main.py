import os
import re
import asyncio
import pathlib

from ollama import AsyncClient


# run `ollama list` to list models
# MODEL = "llama3"
# MODEL = "gemma2:27b"
MODEL = "qwen2.5-coder:7b"


async def send(content, stream=False):
    messages = [
        {
            "role": "user",
            "content": f"以下の質問に日本語で答えてください。英語は使わないでください。日本語のみです。\n\n質問: {content}\n\n回答（日本語のみ）:",
        },
    ]
    return await AsyncClient().chat(model=MODEL, messages=messages, stream=stream)


async def ask_stream(content, handler):
    async for part in await send(content, True):
        answer = part["message"]["content"]
        handler(answer)


async def ask(content):
    response = await send(content)
    return response["message"]["content"]


def print_and_write(s, fopen):
    print(s, end="", flush=True),
    fopen.write(s)


def print_and_collect(s, collector):
    print(s, end="", flush=True),
    collector[0] = collector[0] + s


def remove_line(content, keyword):
    lines = [line for line in content.split("\n") if keyword not in line]
    return "\n".join(lines).strip()


def current_dir():
    return pathlib.Path(os.path.abspath(__file__)).parent


def output_filepath(*paths):
    current = current_dir().joinpath("output")

    path_list = list(paths)
    while len(path_list) >= 1:
        current = current.joinpath(path_list.pop(0))

    current.parent.mkdir(parents=True, exist_ok=True)

    return current


def read_file(filepath):
    with open(filepath, mode="r") as f:
        return f.read()


async def generate_topic(category, count):
    filepath = output_filepath("str_topic.txt")

    if not filepath.exists():
        with open(filepath, mode="w") as f:
            await ask_stream(
                f"私はIT記事のライターです。これから{category}スクリプトのチュートリアルに関するいくつかの記事を書きます。{category}についてのトピックを{count}個教えてください。どんな種類でも構いません。",
                lambda answer: print_and_write(answer, f),
            )
            print()

    return read_file(filepath)


async def extract_topics(category, str_topic, count):
    filepath = output_filepath("topics.txt")

    if not filepath.exists():
        with open(filepath, mode="w") as f:
            await ask_stream(
                f"以下の内容は{category}スクリプトチュートリアルに関する{count}件のトピックです。内容から{count}件のトピックを抽出して正確に出力してください。1行に1つのトピック、インデックス番号、句読点、その他の言葉は使わないでください。\n{str_topic}",
                lambda answer: print_and_write(answer, f),
            )
            print()

    lines = read_file(filepath).split("\n")
    lines = map(lambda line: cleanup_line(line), lines)
    lines = filter(lambda line: line is not None, lines)

    return lines


def cleanup_line(line):
    # Remove leading numbers, dots, dashes, and whitespace
    cleaned = re.sub(r"^\s*\d+[\.\-\)]\s*", "", line.strip())

    # Remove empty lines and common prefixes
    if not cleaned or cleaned.lower().startswith("here are"):
        return None

    return cleaned


async def write_topic(category, topic):
    filepath = output_filepath(category, "article", f"{topic}.md")

    if not filepath.exists():
        print(f"Write an article with topic {topic} ...")

        with open(filepath, mode="w") as f:
            await ask_stream(
                f"{category}のトピック{topic}に関するチュートリアル記事をMarkdown形式で書いてください。できるだけ長く、{category}のスクリプト例があるとより良いです。",
                lambda answer: print_and_write(answer, f),
            )
            print()

    return topic, filepath


async def main():
    category = "Linux"
    topic_count = 50
    str_topic = await generate_topic(category, topic_count)
    topics = await extract_topics(category, str_topic, topic_count)
    for topic in topics:
        topic, filepath = await write_topic(category, topic)


asyncio.run(main())
