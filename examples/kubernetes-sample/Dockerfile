FROM golang:1.19-alpine as build

WORKDIR /src
COPY . ./

WORKDIR /src/examples/kubernetes-sample
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -o app

FROM scratch
COPY --from=build /src/examples/kubernetes-sample/app /
ENTRYPOINT ["/app"]