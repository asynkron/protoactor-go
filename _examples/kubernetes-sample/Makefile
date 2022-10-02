.PHONY: docker docker-local deploy delete redeploy

docker:
	cd ../.. && docker build -f _examples/kubernetes-sample/Dockerfile -t kubernetes-sample:latest .

docker-localcompile:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -o app
	docker build -f Dockerfile-localcompile -t kubernetes-sample:latest .

deploy:
	kubectl apply -f deploy-sample.yaml

delete:
	kubectl delete -f deploy-sample.yaml

redeploy: | delete deploy