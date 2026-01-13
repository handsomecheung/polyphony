FROM	alpine:3.19

RUN	apk update && \
	apk add --no-cache --update aria2 curl && \
	mkdir -p /aria2/conf /aria2/downloads

COPY	conf	/aria2/conf

RUN	chmod +x /aria2/conf/done.sh /aria2/conf/init.sh && \
	echo "bt-tracker=$(curl -s -L 'https://raw.githubusercontent.com/ngosang/trackerslist/master/trackers_all.txt' | sed '/^\s*$/d' | paste -s -d",")" >> /aria2/conf/aria2.conf

CMD	["/aria2/conf/init.sh"]
