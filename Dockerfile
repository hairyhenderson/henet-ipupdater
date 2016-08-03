FROM armhf/alpine:3.4

RUN apk add --no-cache curl
COPY ipupdater.sh /

# amount of time to sleep, in seconds
ENV DELAY 360

CMD [ "/ipupdater.sh" ]
