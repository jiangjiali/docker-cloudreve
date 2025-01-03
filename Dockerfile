FROM amd64/alpine:3.21

ENV APP_UID=1654 \
    PORTS=5212 \
    SERVER=/app

COPY ./bin/cloudreve /root/

RUN set -x \
    && apk add --upgrade --no-cache 'su-exec>=0.2' ca-certificates tzdata libc6-compat libgcc libstdc++ \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo Asia/Shanghai > /etc/timezone \
    && addgroup --gid=$APP_UID app \
    && adduser --uid=$APP_UID --ingroup=app --disabled-password app \
    && mkdir -p $SERVER \
    && mv /root/cloudreve $SERVER/cloudreve \
    && chmod +x $SERVER/cloudreve \
    && mkdir -p $SERVER/logs

WORKDIR $SERVER
VOLUME ["$SERVER/uploads", "$SERVER/avatar", "$SERVER/data"]
EXPOSE $PORTS
