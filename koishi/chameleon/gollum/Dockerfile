FROM python:3.12-alpine

RUN apk add --no-cache git vim ranger && \
	adduser -D -u 1000 gollum

ADD code /code
WORKDIR /code

RUN pip install -r /code/requirements.txt

USER gollum

RUN git config --global --add safe.directory '*' && \
	git config --global user.email "gollum@wiki" && \
	git config --global user.name "Gollum Bot"

CMD ["python", "-u", "main.py"]
