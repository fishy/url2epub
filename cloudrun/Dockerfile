# Start by building the application.
FROM golang:1.19 as build

WORKDIR /go/src/app
COPY . .

RUN go vet ./...
RUN go test ./...

RUN CGO_ENABLED=0 go build -o /go/bin/app

# Now copy it into our base image.
FROM gcr.io/distroless/static
COPY --from=build /go/bin/app /
CMD ["/app"]

