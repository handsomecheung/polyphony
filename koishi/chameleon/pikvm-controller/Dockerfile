FROM python:3.12-alpine

RUN apk add --no-cache openssh-client

ADD code /code
ADD --chmod=755 entrypoint.sh /entrypoint.sh

WORKDIR /code

RUN pip install -r /code/requirements.txt && \
    mkdir -p /code/templates

CMD ["/entrypoint.sh"]
