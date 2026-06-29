FROM golang:1.22-alpine AS build

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/blog-api .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/blog-api /bin/blog-api
EXPOSE 8080
CMD ["/bin/blog-api"]
