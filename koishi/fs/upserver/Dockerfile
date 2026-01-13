FROM python:slim

ADD code/requirements.txt /tmp/requirements.txt
RUN pip install -r /tmp/requirements.txt

ADD code /code

WORKDIR /code
ENTRYPOINT ["python3", "-u", "main.py"]
