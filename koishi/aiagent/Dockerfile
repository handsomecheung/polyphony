FROM python:slim

RUN apt-get update && apt-get install -y \
    curl \
    jq \
    && rm -rf /var/lib/apt/lists/*

RUN pip install uv

ADD code/requirements.txt /tmp/requirements.txt

RUN pip install -r /tmp/requirements.txt

ADD code /code

WORKDIR /code
ENTRYPOINT ["python3", "-u", "main.py", "--serve"]
