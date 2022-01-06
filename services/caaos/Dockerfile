FROM gcr.io/cloud-builders/go:1.17

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download

WORKDIR /workspace/services/caaos
COPY . . 
RUN go build -tags 'netgo,osusergo,static_build' -buildmode pie -ldflags '-s -w -extldflags=-static' -o /caaos

FROM scratch

COPY --from=0 caaos /bin/