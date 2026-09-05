# Build a static binary, then ship it in a distroless image.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dside-events . \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/dside-events /dside-events
COPY --from=build --chown=nonroot:nonroot /out/data /data
ENV ADDR=:8080 DB_PATH=/data/events.db
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/dside-events"]
CMD ["serve"]
