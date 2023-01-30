FROM golang:bullseye

WORKDIR /app
COPY . .

RUN go build -o /usr/local/bin/app
CMD ["sh", "start.sh"]
