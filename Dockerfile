FROM golang:1.20 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

ENV CGO_ENABLED=0

COPY *.go ./
RUN go build -v -o kubevirt-usbdevice-hook

FROM scratch

COPY --from=build /app/kubevirt-usbdevice-hook /kubevirt-usbdevice-hook

ENTRYPOINT ["/kubevirt-usbdevice-hook"]
