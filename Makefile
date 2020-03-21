.PHONY: deps clean build

deps:
	GOPRIVATE=github.com go mod vendor

clean:
	rm -fr ./bin/*

build: clean
	go build -ldflags="-s -w" -o ./bin/proxy ./proxy

zip_handlers: build
	zip -j ./bin/proxy.zip ./bin/proxy
	rm -f ./bin/proxy

package: zip_handlers
	sam package --template-file ./template.yml --output-template-file ./packaged.yml --s3-bucket gapz.deploys

deploy: package
	sam deploy --template-file ./packaged.yml --stack-name Router-Proxy-Stack --parameter-overrides Stage=globe DeployBucket=gapz.deploys --capabilities CAPABILITY_NAMED_IAM
	# deploy to stage
	# change from aws_proxy to aws