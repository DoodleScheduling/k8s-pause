FROM golang:1.21 as builder

WORKDIR /app

COPY . .

RUN make build

FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /app/manager /manager
USER 65532:65532

ENTRYPOINT ["/manager"]
