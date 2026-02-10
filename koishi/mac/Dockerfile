FROM python:3.12-slim-bookworm

ADD code/requirements.txt /tmp/requirements.txt
RUN pip install -r /tmp/requirements.txt

ADD code /data
WORKDIR /data
