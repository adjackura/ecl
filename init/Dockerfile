FROM gcr.io/cloud-builders/go:1.17


WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

WORKDIR /workspace/init
COPY . . 
RUN CGO_ENABLED=0 go build -ldflags '-s -w' -o /init

FROM scratch

COPY --from=0 init /sbin/