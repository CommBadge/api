# syntax=docker/dockerfile:1
FROM golang:1.26-alpine@sha256:70b46548e42db77e0966aaf3619fd068734dc6c77584d526b91126504fd95816 AS build

RUN apk add --no-cache ca-certificates \
    && echo "nobody:x:65534:65534:nobody:/nonexistent:/sbin/nologin" > /etc/passwd \
    && echo "nobody:x:65534:" > /etc/group

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/commbadge-api .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/cert.pem
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group
COPY --from=build /out/commbadge-api /commbadge-api

USER nobody:nobody
ENTRYPOINT ["/commbadge-api"]
