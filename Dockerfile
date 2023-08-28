FROM golang:1.20-alpine

ENV GOOGLE_APPLICATION_CREDENTIALS=./configs/prodServiceAccount.json

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o /sbs-cloud-functions-api

COPY ./configs/prodServiceAccount.json ./configs/prodServiceAccount.json


EXPOSE 8080

CMD [ "/sbs-cloud-functions-api" ]