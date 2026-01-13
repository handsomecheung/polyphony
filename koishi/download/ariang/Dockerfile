FROM alpine:3.19 AS	builder

RUN	apk update && \
	apk add --no-cache --update wget 

RUN	mkdir /AriaNg && \
	VERSION=1.3.7 && \
	wget https://github.com/mayswind/AriaNg/releases/download/${VERSION}/AriaNg-${VERSION}.zip -O /AriaNg.zip && \
	unzip /AriaNg.zip -d /AriaNg


FROM nginx:latest

COPY --from=builder	/AriaNg /AriaNg
ADD	nginx.conf	/etc/nginx/nginx.conf
